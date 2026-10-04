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

	"go-spring.org/cloud/experimental/transaction/tcc"
	"go-spring.org/log"
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
		log.Error(ctx, log.TagAppDef, err, log.Msg("tcc recovery: scanning pending transactions failed"))
		return nil
	}
	for _, snap := range pending {
		// Every line about this transaction names it from here on: the ID rides
		// on a context minted per entry, so it never leaks into the next
		// transaction's lines.
		tccCtx := log.WithFields(ctx, log.String("transaction", snap.ID))

		parts, ok := r.Registry.Lookup(snap.Method)
		if !ok {
			// Recovery depends on the participants being registered at wiring time
			// under the same method name GlobalTCC recorded; without them the
			// transaction cannot be rebuilt.
			log.Warn(tccCtx, log.TagAppDef,
				log.String("method", snap.Method),
				log.Msg("tcc recovery: skip method with no registered participants"))
			continue
		}
		res, err := r.Coord.Recover(tccCtx, tcc.Transaction{ID: snap.ID, Method: snap.Method, Participants: parts})
		if err != nil {
			log.Error(tccCtx, log.TagAppDef, err, log.Msg("tcc recovery: recovering transaction failed"))
			continue
		}
		log.Info(tccCtx, log.TagAppDef,
			log.String("status", res.Status.String()),
			log.Msg("tcc recovery: transaction recovered"))
	}
	return nil
}
