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

package StarterTransactionTCC

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/stdlib/transaction/tcc"
)

// recoveryRunner drives transactions a crash left in flight to their decided
// outcome. As a gs.Runner it executes once, synchronously, during application
// startup — after wiring, so the ParticipantRegistry is already populated. It
// scans the durable Store for non-terminal snapshots and, for each, rebuilds the
// transaction's participants from the registry (keyed by the persisted method
// name) and hands them to the coordinator: forward confirm if a commit decision
// was recorded, otherwise backward cancel.
//
// It is a harmless no-op under the in-memory default Store, whose Pending is
// always empty after a restart.
type recoveryRunner struct {
	Store    tcc.Store                `autowire:""`
	Registry *tcc.ParticipantRegistry `autowire:""`
	Coord    tcc.Coordinator          `autowire:""`
}

// newRecoveryRunner is the constructor registered with the container; the
// dependencies are populated by the autowire tags.
func newRecoveryRunner() *recoveryRunner { return &recoveryRunner{} }

// Run scans for interrupted transactions and recovers each. It never fails
// startup: a transaction whose participants are no longer registered is logged
// and skipped (its definition must be re-declared for recovery to act), and an
// individual recovery error is logged rather than aborting the remaining
// transactions.
func (r *recoveryRunner) Run(ctx context.Context) error {
	pending, err := r.Store.Pending(ctx)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "tcc recovery: scanning pending transactions failed: %v", err)
		return nil
	}
	for _, snap := range pending {
		parts, ok := r.Registry.Lookup(snap.Method)
		if !ok {
			// Recovery depends on the participants being registered at wiring time
			// under the same method name GlobalTCC recorded; without them the
			// transaction cannot be rebuilt.
			log.Warnf(ctx, log.TagAppDef,
				"tcc recovery: no participants registered for method %q (transaction %q); skipping", snap.Method, snap.ID)
			continue
		}
		res, err := r.Coord.Recover(ctx, tcc.Transaction{ID: snap.ID, Method: snap.Method, Participants: parts})
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "tcc recovery: recovering transaction %q failed: %v", snap.ID, err)
			continue
		}
		log.Infof(ctx, log.TagAppDef, "tcc recovery: transaction %q recovered with status %s", snap.ID, res.Status)
	}
	return nil
}
