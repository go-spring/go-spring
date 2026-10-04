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
	"fmt"
	"go-spring.org/stdlib/errutil"
	"time"

	"go-spring.org/cloud/experimental/transaction/at"
	"go-spring.org/log"
)

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
// The enrolled branches come from the coordinator bean: each plugin enrolled
// itself with [at.Coordinator.Enroll] at Initialize time (during wiring), and
// this Runner executes after wiring, so the list is complete. This is why
// applications MUST install the plugin during bean construction (as the example
// does), not from inside a later Runner.
//
// Scanning at boot is unambiguous in this starter's single-process model: no
// transaction of this process can be in flight before Runners execute, so any
// undo-log row found is an orphan. Sharing the database between processes is
// outside that contract — such deployments must set
// spring.transaction.at.recover-on-start=false and reconcile manually.
type recoveryRunner struct {
	coord at.Coordinator
}

// newRecoveryRunner is the constructor registered with the container.
func newRecoveryRunner(coord at.Coordinator) *recoveryRunner {
	return &recoveryRunner{coord: coord}
}

// Run recovers every enrolled database. It never fails startup: a database that
// cannot be scanned or an xid whose replay fails is logged (ERROR) and skipped,
// leaving its undo logs in place for manual recovery, so one bad database does
// not block the rest of the application.
func (r *recoveryRunner) Run(ctx context.Context) error {
	for _, b := range r.coord.Enrolled() {
		_ = recoverBranch(ctx, b)
	}
	return nil
}

// recoverBranch scans one enrolled database for orphaned undo logs and replays
// them. It logs a loud ERROR when orphans exist (count plus the oldest entry,
// the two numbers an operator needs to gauge exposure) before attempting
// recovery, so a failed replay has already been reported. A branch this
// gorm-specific starter does not recognize is logged and skipped.
func recoverBranch(ctx context.Context, b at.Branch) error {
	gb, ok := b.(*gormBranch)
	if !ok {
		err := errutil.Explain(nil, "at recovery: branch %s has unsupported type %T", b.ID(), b)
		log.Error(ctx, log.TagAppDef, err,
			log.String("branch", b.ID()),
			log.String("type", fmt.Sprintf("%T", b)),
			log.Msg("at recovery: skip non-gorm branch"))
		return nil
	}
	resource, db := gb.resource, gb.db

	// The branch's resource names every line below: it rides on the context so
	// the recovery trail stays greppable per resource without repeating it.
	ctx = log.WithFields(ctx, log.String("resource", resource))

	var xids []string
	if err := db.Model(&undoRow{}).Distinct().Order("xid").
		Pluck("xid", &xids).Error; err != nil {
		log.Error(ctx, log.TagAppDef, err,
			log.Msg("at recovery: scanning undo logs failed (orphaned entries, if any, are left for manual recovery)"))
		return err
	}
	if len(xids) == 0 {
		log.Debug(ctx, log.TagAppDef, func() []log.Field {
			return []log.Field{log.Msg("at recovery: no orphaned undo logs")}
		})
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
	err := errutil.Explain(nil, "found %d orphaned undo-log entries from a previous run", count)
	log.Error(ctx, log.TagAppDef, err,
		log.Int("entries", count),
		log.String("oldest", oldest.Format(time.RFC3339)),
		log.Int("transactions", len(xids)),
		log.Msg("at recovery: found orphaned undo-log entries from a previous run; rolling back"))

	for _, xid := range xids {
		if err := gb.Rollback(ctx, xid); err != nil {
			log.Error(ctx, log.TagAppDef, err,
				log.String("xid", xid),
				log.Msg("at recovery: rolling back global transaction failed (undo logs kept for manual recovery)"))
			continue
		}
		log.Info(ctx, log.TagAppDef,
			log.String("xid", xid),
			log.Msg("at recovery: global transaction rolled back"))
	}
	return nil
}
