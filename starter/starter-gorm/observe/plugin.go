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
// go-spring gorm starters. It DECLARES what one Create/Query/Update/Delete
// operation IS — its name, its db.system/db.operation labels, its SQL statement
// and its access tag — on the call's context via [observability.WithOperation];
// it emits nothing itself.
//
//	db.Use(gormobserve.NewPlugin("mysql"))
//
// The signals — the call span, the call- and attempt-level duration histograms,
// the in-flight gauge and the one access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included); see cloud/resilience/observe.go. Declaring rather than
// emitting is what the plugin alone can do: only it knows these calls reach a
// database through gorm, and only it knows the operation's kind.
package gormobservability

import (
	"context"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"gorm.io/gorm"
)

// accessTag is the static log tag for the gorm access log; the engine is a log
// field, not part of the tag. It is registered here, at package init, because a
// tag must exist before the framework's first property refresh — see
// [log.RegisterTag].
var accessTag = log.RegisterAppTag("gorm", "access")

// maxStatement bounds the SQL captured as db.statement. A statement can be long
// and a span or a log line has no use for all of it.
const maxStatement = 512

// operation is the semantic identity of one gorm operation. kind names the
// operation ("create"/"query"/"update"/"delete"); stmt is the SQL gorm built
// for it, empty when the operation produced none.
//
// The statement rides in Detail rather than Attrs: SQL is drawn from an open
// set, so as a metric label it would multiply the series without bound. Detail
// reaches the span and the log — where the statement is exactly what makes a
// line worth reading — and never a label. A statement-less operation carries no
// detail at all, which is also what levels its success log at Info.
func operation(system, kind, stmt string) observability.Operation {
	o := observability.Operation{
		Name:   kind,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", system),
			attribute.String("db.operation", kind),
		},
		LogTag: accessTag,
	}
	if stmt != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("db.statement", strutil.Truncate(stmt, maxStatement)),
		}
	}
	return o
}

// observePlugin is a gorm Plugin that declares every Create/Query/Update/Delete
// as an operation. It runs at the gorm:<op> anchor's Before position, which is
// the last point before the resilience layer (which wraps that same processor)
// takes over — so the declaration is what the emitter below reads.
//
// Only the four data processors are declared; gorm's noisy internal callbacks
// (row processing, transaction bookkeeping) are not operations here, so no
// op-skip list is needed.
type observePlugin struct {
	system string

	// orig holds the dialect's own per-operation processor, captured in
	// [observePlugin.Initialize] before anything replaces it — the resilience
	// callbacks do, and after that the anchor named gorm:<op> is the wrapper,
	// not the SQL builder. [observePlugin.statement] dry-runs it to learn the SQL
	// the operation is about to build.
	orig map[string]func(*gorm.DB)
}

// NewPlugin builds a gorm.Plugin that declares every operation's identity under
// the given db.system label (e.g. "mysql", "postgresql", "clickhouse",
// "microsoft.sql_server"). The signals themselves are emitted by the resilience
// layer from the declared operation.
func NewPlugin(system string) gorm.Plugin {
	return &observePlugin{system: system}
}

func (p *observePlugin) Name() string { return "go-spring:observe" }

func (p *observePlugin) Initialize(db *gorm.DB) error {
	// Capture the dialect's own processors here, before the resilience callbacks
	// replace the gorm:<op> anchors: the SQL the plugin declares is the one the
	// real run then reuses, built by exactly this function.
	p.orig = map[string]func(*gorm.DB){
		"create": db.Callback().Create().Get("gorm:create"),
		"query":  db.Callback().Query().Get("gorm:query"),
		"update": db.Callback().Update().Get("gorm:update"),
		"delete": db.Callback().Delete().Get("gorm:delete"),
	}

	ops := []struct {
		kind string
		reg  func(fn func(*gorm.DB)) error
	}{
		{"create", func(fn func(*gorm.DB)) error {
			return db.Callback().Create().Before("gorm:create").Register("go-spring:observe:declare_create", fn)
		}},
		{"query", func(fn func(*gorm.DB)) error {
			return db.Callback().Query().Before("gorm:query").Register("go-spring:observe:declare_query", fn)
		}},
		{"update", func(fn func(*gorm.DB)) error {
			return db.Callback().Update().Before("gorm:update").Register("go-spring:observe:declare_update", fn)
		}},
		{"delete", func(fn func(*gorm.DB)) error {
			return db.Callback().Delete().Before("gorm:delete").Register("go-spring:observe:declare_delete", fn)
		}},
	}
	for _, op := range ops {
		kind := op.kind
		before := func(tx *gorm.DB) { p.declare(tx, kind) }
		if err := op.reg(before); err != nil {
			return err
		}
	}
	return nil
}

// declare puts the operation's identity on the call's context, where the
// resilience layer wrapping the gorm:<op> processor below reads it back to emit
// the call's span, metrics and access log.
func (p *observePlugin) declare(tx *gorm.DB, kind string) {
	if tx.Statement == nil {
		return
	}
	ctx := tx.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	tx.Statement.Context = observability.WithOperation(ctx, operation(p.system, kind, p.statement(tx, kind)))
}

// statement returns the SQL the operation is about to build, without executing
// it. gorm builds the SQL inside the very processor the resilience layer wraps,
// so it is not known yet at this point; the plugin dry-runs the dialect's own
// processor on a session that shares this statement. The build runs, the
// dry-run stops before touching the driver, and the real run below then finds
// the SQL already built and reuses it — so the statement is built exactly once,
// by gorm's own code, not by a copy of it. An unknown processor yields no
// statement, and the operation is declared without detail.
func (p *observePlugin) statement(tx *gorm.DB, kind string) string {
	orig := p.orig[kind]
	if orig == nil {
		return ""
	}
	orig(tx.Session(&gorm.Session{DryRun: true}))
	return tx.Statement.SQL.String()
}
