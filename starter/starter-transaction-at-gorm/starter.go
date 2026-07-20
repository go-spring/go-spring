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

// Package StarterTransactionATGorm contributes the AT (Automatic Transaction)
// distributed-transaction capability defined in
// [go-spring.org/spring/transaction/at] to a Go-Spring application, backed by
// gorm. It is enabled by a blank import:
//
//	import _ "go-spring.org/starter-transaction-at-gorm"
//
// After that the container holds two beans:
//
//   - an at.GlobalLock — the in-memory global row lock providing write-write
//     isolation between concurrent global transactions;
//   - an at.Coordinator — the bundled in-process orchestrator that, on success,
//     drops every branch's undo logs (second-phase commit) and, on failure,
//     restores every changed row from its recorded before-image (second-phase
//     rollback).
//
// Unlike Saga and TCC, AT needs no business-written compensation and no method
// registry: compensation is derived automatically from the before-image that a
// gorm plugin captures around each DML statement. Wiring is therefore two steps
// per database — migrate the undo-log table and install the plugin:
//
//	if err := atgorm.Migrate(db); err != nil { ... }
//	if err := db.Use(atgorm.NewPlugin("account-db", coord, lock)); err != nil { ... }
//
// then run business writes inside an [at.GlobalAT] aspect (or an explicit
// coord.Begin/Commit/Rollback) so the plugin sees the global transaction id on
// the context and self-registers a branch.
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it opens
// no port and starts no server. It is the transparency-first sibling of
// starter-transaction-saga and starter-transaction-tcc — reach for AT when the
// data lives in a SQL database and you want automatic rollback without writing
// compensation, for TCC when a resource must be reserved between try and commit,
// and for Saga when each step's effect is real and undone by explicit
// compensation.
//
// The global lock and undo log are process-local: this is a single-process AT
// equivalent, not a distributed Seata TC/TM/RM deployment.
//
// Observability is on by default: when Config.Tracing is true the coordinator
// emits an otel child span per branch phase on the globals starter-otel installs.
package StarterTransactionATGorm

import (
	"go-spring.org/spring/gs"
	"go-spring.org/spring/transaction/at"
)

// enabled matches when the starter is not explicitly disabled.
var enabled = gs.OnProperty("spring.transaction.at.enabled").HavingValue("true").MatchIfMissing()

func init() {
	// The default global row lock: in-memory, so the framework stays standalone. It
	// steps aside (OnMissingBean) the moment a durable-lock starter contributes its
	// own at.GlobalLock. Exported under the interface so both the coordinator and
	// application plugin installation depend on the abstraction.
	gs.Provide(func() at.GlobalLock { return &at.MemoryGlobalLock{} }).
		Condition(enabled, gs.OnMissingBean[at.GlobalLock]()).
		Export(gs.As[at.GlobalLock]())

	// The in-process coordinator, built from the bound configuration (for the
	// tracing toggle) and the autowired GlobalLock. Exported as the at.Coordinator
	// interface so business code depends on the abstraction, not this construction.
	gs.Provide(newCoordinator, gs.TagArg("${spring.transaction.at}"), gs.TagArg("")).
		Condition(enabled).
		Export(gs.As[at.Coordinator]())
}

// newCoordinator builds the bundled in-process coordinator over the autowired
// global lock and, when tracing is enabled, the otel observer.
func newCoordinator(c Config, lock at.GlobalLock) at.Coordinator {
	opts := []at.Option{at.WithGlobalLock(lock)}
	if c.Tracing {
		opts = append(opts, at.WithObserver(otelObserver{}))
	}
	return at.NewCoordinator(opts...)
}
