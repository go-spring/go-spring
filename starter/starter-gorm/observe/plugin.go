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

// Package gormobservability is the GORM instrumentation plugin shared by the
// go-spring gorm starters. It BUILDS what one Create/Query/Update/Delete
// operation IS — its name, its db.system/db.operation labels, its SQL statement
// and its access tag — as a [chain.Operation]; it emits nothing itself.
//
//	db.Use(gormobserve.NewPlugin("mysql"))
//
// The signals — the call span, the call- and attempt-level duration histograms,
// the in-flight gauge and the one access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included); see cloud/governance/resilience/observe.go. The identity now travels as an
// explicit argument to the executor rather than on the call's context: the
// resilience callback that wraps each gorm processor asks this plugin for the
// operation ([Plugin.Operation]) and passes it to resilience.Run. Building
// rather than emitting is what the plugin alone can do: only it knows these
// calls reach a database through gorm, and only it knows the operation's kind.
package gormobservability

import (
	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"gorm.io/gorm"
)

// accessTag is the static log tag for the gorm access log; the engine is a log
// field, not part of the tag. It is registered here, at package init, because a
// tag must exist before the framework's first property refresh — see
// [log.RegisterTag].
var accessTag = log.RegisterAppTag("gorm", "access")

// operation is the semantic identity of one gorm operation. kind names the
// operation ("create"/"query"/"update"/"delete"); stmt is the SQL gorm built
// for it, empty when the operation produced none. The family's vocabulary —
// attribute keys, the bounded/open split that puts the SQL in detail, the
// truncation bound — lives in [wire].
func operation(system, kind, stmt string) chain.Operation {
	o := chain.DBOp(system, kind, stmt)
	o.Observe.Log.Tag = accessTag
	return o
}

// Plugin is a gorm Plugin that supplies the identity of every
// Create/Query/Update/Delete operation. It captures the dialect's own
// per-operation processors in [Plugin.Initialize], before the resilience layer
// replaces the gorm:<op> anchors — so [Plugin.Operation] can dry-run the real
// SQL builder to learn the SQL an operation is about to build.
//
// Only the four data processors are supplied; gorm's noisy internal callbacks
// (row processing, transaction bookkeeping) are not operations here, so
// [Plugin.Operation] returns the zero Operation for them and the resilience
// layer reports them as any other undeclared call.
type Plugin struct {
	system string

	// orig holds the dialect's own per-operation processor, captured in
	// [Plugin.Initialize] before anything replaces it — the resilience callbacks
	// do, and after that the anchor named gorm:<op> is the wrapper, not the SQL
	// builder. [Plugin.Operation] dry-runs it to learn the SQL the operation is
	// about to build.
	orig map[string]func(*gorm.DB)
}

// NewPlugin builds a [Plugin] that supplies every operation's identity under the
// given db.system label (e.g. "mysql", "postgresql", "clickhouse",
// "microsoft.sql_server"). The signals themselves are emitted by the resilience
// layer from the supplied operation.
func NewPlugin(system string) *Plugin {
	return &Plugin{system: system}
}

func (p *Plugin) Name() string { return "go-spring:observe" }

func (p *Plugin) Initialize(db *gorm.DB) error {
	// Capture the dialect's own processors here, before the resilience callbacks
	// replace the gorm:<op> anchors: the SQL the plugin supplies is the one the
	// real run then reuses, built by exactly this function.
	p.orig = map[string]func(*gorm.DB){
		"create": db.Callback().Create().Get("gorm:create"),
		"query":  db.Callback().Query().Get("gorm:query"),
		"update": db.Callback().Update().Get("gorm:update"),
		"delete": db.Callback().Delete().Get("gorm:delete"),
	}
	return nil
}

// Operation returns the semantic identity of the gorm operation kind is about to
// run, for the resilience layer to pass explicitly to the executor. It returns
// the zero [chain.Operation] for a kind this plugin does not supply (gorm:row,
// gorm:raw), so the resilience layer reports those as any other call whose
// identity is undeclared.
//
// The SQL the operation is about to build is not known yet — gorm builds it
// inside the very processor the resilience layer wraps — so the plugin dry-runs
// the dialect's own processor on a session that shares this statement. The build
// runs, the dry-run stops before touching the driver, and the real run then
// finds the SQL already built and reuses it — so the statement is built exactly
// once, by gorm's own code, not by a copy of it. An unknown processor yields no
// statement, and the operation is built without detail.
func (p *Plugin) Operation(tx *gorm.DB, kind string) chain.Operation {
	if tx == nil || tx.Statement == nil {
		return chain.Operation{}
	}
	orig := p.orig[kind]
	if orig == nil {
		return chain.Operation{}
	}
	orig(tx.Session(&gorm.Session{DryRun: true}))
	return operation(p.system, kind, tx.Statement.SQL.String())
}
