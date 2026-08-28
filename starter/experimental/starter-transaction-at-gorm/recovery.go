/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package StarterTransactionATGorm

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"go-spring.org/log"
	"gorm.io/gorm"
)

// The recovery Runner needs a *gorm.DB handle per AT-enrolled database, but the
// plugin (not the starter) owns those handles: the application installs it with
// db.Use during wiring. So each plugin registers its base handle here at
// Initialize time, and the Runner — which executes after wiring — iterates the
// registry. This is why applications MUST install the plugin during bean
// construction (as the example does), not from inside a later Runner.
var (
	recoverMu  sync.Mutex
	recoverDBs = map[string]*gorm.DB{} // resource id -> base handle
)

// registerBranchDB records the base handle of one AT-enrolled database for the
// startup recovery scan.
func registerBranchDB(resource string, db *gorm.DB) {
	recoverMu.Lock()
	defer recoverMu.Unlock()
	recoverDBs[resource] = db
}

// recoveryRunner replays, at startup, the undo logs a crash left behind. AT is
// two-phase: phase one commits the business data together with its undo log, and
// phase two either drops the undo log (commit) or restores the before-image and
// drops it (rollback). If the process dies between the phases, the undo-log rows
// survive while the in-memory coordinator — the only record of the global
// transaction — is gone: the transaction can never commit, so rollback is the
// only completable outcome (Seata resolves undecided AT transactions the same
// way). The Runner therefore scans each enrolled database's at_undo_log table
// and replays every orphan's undo.
//
// Scanning at boot is unambiguous in this starter's single-process model: no
// transaction of this process can be in flight before Runners execute, so any
// undo-log row found is an orphan. Sharing the database between processes is
// outside that contract — such deployments must set
// spring.transaction.at.recover-on-start=false and reconcile manually.
type recoveryRunner struct{}

// newRecoveryRunner is the constructor registered with the container.
func newRecoveryRunner() *recoveryRunner { return &recoveryRunner{} }

// Run recovers every enrolled database. It never fails startup: a database that
// cannot be scanned or an xid whose replay fails is logged (ERROR) and skipped,
// leaving its undo logs in place for manual recovery, so one bad database does
// not block the rest of the application.
func (r *recoveryRunner) Run(ctx context.Context) error {
	recoverMu.Lock()
	resources := slices.Sorted(maps.Keys(recoverDBs))
	dbs := make([]*gorm.DB, len(resources))
	for i, res := range resources {
		dbs[i] = recoverDBs[res]
	}
	recoverMu.Unlock()

	for i, db := range dbs {
		_ = recoverDatabase(ctx, resources[i], db)
	}
	return nil
}

// recoverDatabase scans one database for orphaned undo logs and replays them.
// It logs a loud ERROR when orphans exist (count plus the oldest entry, the two
// numbers an operator needs to gauge exposure) before attempting recovery, so a
// failed replay has already been reported.
func recoverDatabase(ctx context.Context, resource string, db *gorm.DB) error {
	var xids []string
	if err := db.Model(&undoRow{}).Distinct().Order("xid").
		Pluck("xid", &xids).Error; err != nil {
		log.Errorf(ctx, log.TagAppDef,
			"at recovery: scanning undo logs on resource %q failed: %v (orphaned entries, if any, are left for manual recovery)", resource, err)
		return err
	}
	if len(xids) == 0 {
		log.Debugf(ctx, log.TagAppDef, "at recovery: no orphaned undo logs on resource %q", resource)
		return nil
	}

	var count int64
	var oldest time.Time
	if err := db.Model(&undoRow{}).Count(&count).Error; err == nil {
		oldest = time.Now()
		var first undoRow
		if err := db.Order("id").First(&first).Error; err == nil {
			oldest = first.CreatedAt
		}
	}
	log.Errorf(ctx, log.TagAppDef,
		"at recovery: found %d orphaned undo-log entries on resource %q (oldest %s) from %d interrupted global transaction(s) of a previous run; rolling back",
		count, resource, oldest.Format(time.RFC3339), len(xids))

	b := &gormBranch{resource: resource, db: db}
	for _, xid := range xids {
		if err := b.Rollback(ctx, xid); err != nil {
			log.Errorf(ctx, log.TagAppDef,
				"at recovery: rolling back global transaction %q on resource %q failed: %v (undo logs kept for manual recovery)", xid, resource, err)
			continue
		}
		log.Infof(ctx, log.TagAppDef,
			"at recovery: global transaction %q rolled back on resource %q", xid, resource)
	}
	return nil
}
