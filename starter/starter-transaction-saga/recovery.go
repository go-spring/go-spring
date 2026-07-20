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

package StarterTransactionSaga

import (
	"context"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/transaction"
)

// recoveryRunner compensates sagas a crash left in flight. As a gs.Runner it
// executes once, synchronously, during application startup — after wiring, so
// the StepRegistry is already populated. It scans the durable Store for
// StatusRunning snapshots and, for each, rebuilds the saga's steps from the
// registry (keyed by the persisted method name) and hands them to the
// coordinator for backward recovery.
//
// It is a harmless no-op under the in-memory default Store, whose Pending is
// always empty after a restart.
type recoveryRunner struct {
	Store    transaction.Store         `autowire:""`
	Registry *transaction.StepRegistry `autowire:""`
	Coord    transaction.Coordinator   `autowire:""`
}

// newRecoveryRunner is the constructor registered with the container; the
// dependencies are populated by the autowire tags.
func newRecoveryRunner() *recoveryRunner { return &recoveryRunner{} }

// Run scans for interrupted sagas and recovers each. It never fails startup: a
// saga whose steps are no longer registered is logged and skipped (its
// definition must be re-declared for recovery to act), and an individual
// recovery error is logged rather than aborting the remaining sagas.
func (r *recoveryRunner) Run(ctx context.Context) error {
	pending, err := r.Store.Pending(ctx)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "saga recovery: scanning pending sagas failed: %v", err)
		return nil
	}
	for _, snap := range pending {
		steps, ok := r.Registry.Lookup(snap.Method)
		if !ok {
			// Recovery depends on the steps being registered at wiring time under
			// the same method name GlobalTransactional recorded; without them the
			// saga cannot be rebuilt.
			log.Warnf(ctx, log.TagAppDef,
				"saga recovery: no steps registered for method %q (saga %q); skipping", snap.Method, snap.ID)
			continue
		}
		res, err := r.Coord.Recover(ctx, transaction.Saga{ID: snap.ID, Method: snap.Method, Steps: steps})
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "saga recovery: recovering saga %q failed: %v", snap.ID, err)
			continue
		}
		log.Infof(ctx, log.TagAppDef, "saga recovery: saga %q recovered with status %s", snap.ID, res.Status)
	}
	return nil
}
