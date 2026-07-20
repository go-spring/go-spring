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

// Package StarterTransactionTCC contributes the TCC (Try / Confirm / Cancel)
// distributed-transaction capability defined in
// [go-spring.org/spring/transaction/tcc] to a Go-Spring application. It is
// enabled by a blank import:
//
//	import _ "go-spring.org/starter-transaction-tcc"
//
// After that the container holds two beans:
//
//   - a tcc.Coordinator — the bundled in-process orchestrator that tries every
//     participant and then confirms all of them, or cancels the tried ones on any
//     try failure;
//   - a *tcc.ParticipantRegistry — where application code registers each
//     TCC method's participants.
//
// Wire them into an aspect chain to get the @GlobalTransactional(TCC) effect:
//
//	func RegisterOrder(reg *tcc.ParticipantRegistry) {
//	    reg.Register("OrderService.Place", reserveStock, freezeBalance)
//	}
//	chain := aspect.NewChain(tcc.GlobalTCC(coord, reg))
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it opens
// no port and starts no server. It is the strong-consistency sibling of
// starter-transaction-saga — reach for TCC when a resource must be reserved
// (invisible to other readers) between the try and the commit, and for Saga when
// each step's effect is real and undone by compensation.
//
// By default the TCC log is kept in memory only — enough for the common
// single-process case and for tests, but NOT crash-recoverable. Importing a
// durable-Store starter (spring.transaction.tcc.store=...) supplies a tcc.Store
// bean; because the in-memory default is registered with gs.OnMissingBean, that
// durable Store then takes over both the coordinator and the startup recovery
// scan.
//
// When Config.RecoverOnStart is true (the default) a gs.Runner scans the Store at
// startup and drives any transaction a crash left in flight to its decided
// outcome. Recovery rebuilds each transaction's participants from the
// ParticipantRegistry keyed by the persisted method name, so applications MUST
// register their participants at wiring time (bean construction), not from inside
// a custom Runner — otherwise the participants may be absent when the recovery
// Runner runs.
//
// Observability is on by default: when Config.Tracing is true the coordinator
// emits an otel child span per participant phase on the globals starter-otel
// installs.
package StarterTransactionTCC

import (
	"go-spring.org/spring/gs"
	"go-spring.org/spring/transaction/tcc"
)

// enabled matches when the starter is not explicitly disabled.
var enabled = gs.OnProperty("spring.transaction.tcc.enabled").HavingValue("true").MatchIfMissing()

// recoverOnStart matches when startup recovery is not explicitly disabled.
var recoverOnStart = gs.OnProperty("spring.transaction.tcc.recover-on-start").HavingValue("true").MatchIfMissing()

func init() {
	// The registry where application code declares each method's TCC participants.
	// It is a shared, concurrency-safe map, so a single instance serves the app.
	gs.Provide(tcc.NewParticipantRegistry).
		Condition(enabled)

	// The default TCC-log Store: in-memory, so the framework stays standalone. It
	// steps aside (OnMissingBean) the moment a durable-Store starter contributes
	// its own tcc.Store, which is how crash recovery is switched on without
	// changing this module.
	gs.Provide(func() tcc.Store { return &tcc.MemoryStore{} }).
		Condition(enabled, gs.OnMissingBean[tcc.Store]())

	// The in-process coordinator, built from the bound configuration (for the
	// tracing toggle) and the autowired Store. Exported as the tcc.Coordinator
	// interface so business code depends on the abstraction, not this construction.
	gs.Provide(newCoordinator, gs.TagArg("${spring.transaction.tcc}"), gs.TagArg("")).
		Condition(enabled).
		Export(gs.As[tcc.Coordinator]())

	// The startup recovery Runner. It is a no-op under the in-memory default Store
	// (Pending is always empty after a restart) and does real work only with a
	// durable Store.
	gs.Provide(newRecoveryRunner).
		Condition(enabled, recoverOnStart).
		Export(gs.As[gs.Runner]())
}

// newCoordinator builds the bundled in-process coordinator over the autowired
// TCC-log Store and, when tracing is enabled, the otel observer.
func newCoordinator(c Config, store tcc.Store) tcc.Coordinator {
	opts := []tcc.Option{tcc.WithStore(store)}
	if c.Tracing {
		opts = append(opts, tcc.WithObserver(otelObserver{}))
	}
	return tcc.NewCoordinator(opts...)
}
