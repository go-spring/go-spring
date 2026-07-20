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

// Package StarterTransactionSaga contributes the Saga distributed-transaction
// capability defined in [go-spring.org/spring/transaction] to a Go-Spring
// application. It is enabled by a blank import:
//
//	import _ "go-spring.org/starter-transaction-saga"
//
// After that the container holds two beans:
//
//   - a transaction.Coordinator — the bundled in-process orchestrator that runs
//     a Saga's steps and compensates in reverse on failure;
//   - a *transaction.StepRegistry — where application code registers each
//     @GlobalTransactional-equivalent method's steps.
//
// Wire them into an aspect chain to get the @GlobalTransactional effect:
//
//	func RegisterOrder(reg *transaction.StepRegistry) {
//	    reg.Register("OrderService.Place", deductInventory, chargePayment, publishEvent)
//	}
//	chain := aspect.NewChain(transaction.GlobalTransactional(coord, reg))
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it opens
// no port and starts no server. By default the saga log is kept in memory only —
// enough for the common "one gateway process drives several downstreams" case
// and for tests, but NOT crash-recoverable. Importing a durable-Store starter
// (e.g. starter-transaction-saga-gorm with spring.transaction.saga.store=gorm)
// supplies a transaction.Store bean; because the in-memory default is registered
// with gs.OnMissingBean, that durable Store then takes over both the coordinator
// and the startup recovery scan.
//
// When Config.RecoverOnStart is true (the default) a gs.Runner scans the Store at
// startup and compensates any saga a crash left in flight. Recovery rebuilds each
// saga's steps from the StepRegistry keyed by the persisted method name, so
// applications MUST register their steps at wiring time (bean construction), not
// from inside a custom Runner — otherwise the steps may be absent when the
// recovery Runner runs.
//
// Observability is on by default: when Config.Tracing is true the coordinator
// emits an otel child span per step phase on the globals starter-otel installs.
package StarterTransactionSaga

import (
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/transaction"
)

// enabled matches when the starter is not explicitly disabled.
var enabled = gs.OnProperty("spring.transaction.saga.enabled").HavingValue("true").MatchIfMissing()

// recoverOnStart matches when startup recovery is not explicitly disabled.
var recoverOnStart = gs.OnProperty("spring.transaction.saga.recover-on-start").HavingValue("true").MatchIfMissing()

func init() {
	// The registry where application code declares each method's Saga steps. It is
	// a shared, concurrency-safe map, so a single instance serves the whole app.
	gs.Provide(transaction.NewStepRegistry).
		Condition(enabled)

	// The default saga-log Store: in-memory, so the framework stays standalone.
	// It steps aside (OnMissingBean) the moment a durable-Store starter contributes
	// its own transaction.Store, which is how crash recovery is switched on without
	// changing this module.
	gs.Provide(func() transaction.Store { return &transaction.MemoryStore{} }).
		Condition(enabled, gs.OnMissingBean[transaction.Store]())

	// The in-process coordinator, built from the bound configuration (for the
	// tracing toggle) and the autowired Store. Exported as the
	// transaction.Coordinator interface so business code depends on the
	// abstraction, not this construction.
	gs.Provide(newCoordinator, gs.TagArg("${spring.transaction.saga}"), gs.TagArg("")).
		Condition(enabled).
		Export(gs.As[transaction.Coordinator]())

	// The startup recovery Runner. It is a no-op under the in-memory default Store
	// (Pending is always empty after a restart) and does real work only with a
	// durable Store.
	gs.Provide(newRecoveryRunner).
		Condition(enabled, recoverOnStart).
		Export(gs.As[gs.Runner]())
}

// newCoordinator builds the bundled in-process coordinator over the autowired
// saga-log Store and, when tracing is enabled, the otel observer.
func newCoordinator(c Config, store transaction.Store) transaction.Coordinator {
	opts := []transaction.Option{transaction.WithStore(store)}
	if c.Tracing {
		opts = append(opts, transaction.WithObserver(otelObserver{}))
	}
	return transaction.NewCoordinator(opts...)
}
