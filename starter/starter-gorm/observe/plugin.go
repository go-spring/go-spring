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
// go-spring gorm starters. It emits one client span (db.system/db.operation/
// db.statement), one db.client.operation.duration record, and one access-log
// line for every Create/Query/Update/Delete via a gorm.Plugin:
//
//	db.Use(gormobserve.NewPlugin("mysql"))
//
// The signals ride the OTel globals: without starter-otel the tracer and meter
// are no-ops, so the plugin adds near-zero overhead and is installed
// unconditionally.
package gormobservability

import (
	"context"
	"go-spring.org/stdlib/strutil"
	"sync"
	"time"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// durationBuckets are the duration-histogram boundaries (seconds) — the OTel
// HTTP semconv recommended set.
var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}
var (
	// accessTag is the static log tag for the gorm access log; the engine is a
	// log field, not part of the tag.
	accessTag = log.RegisterAppTag("gorm", "access")

	tracer = otel.Tracer("go-spring.org/starter-gorm/observe")
)

// newDuration builds the db.client.operation.duration histogram from whatever
// meter provider is current — created per plugin, not at package init, so an
// SDK installed later than this package's init still receives the records.
func newDuration() metric.Float64Histogram {
	h, _ := otel.Meter("go-spring.org/starter-gorm/observe").Float64Histogram("db.client.operation.duration",
		metric.WithDescription("Duration of gorm client operations"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(durationBuckets...))
	return h
}

// gormSpans correlates a gorm operation's Before callback (which opens the
// span for timing) with its After callback (which ends it). gorm hands the
// same *gorm.DB to both, and creates a fresh one per operation, so the pointer
// is a unique key for one in-flight operation.
var gormSpans sync.Map // *gorm.DB -> *opSpan

// observePlugin is a gorm Plugin that instruments every Create/Query/Update/
// Delete. The SQL statement is not known until the After callback (gorm builds
// it during the gorm:<op> processor that runs between Before and After), so the
// Before callback opens the span for timing and the After callback calls SetArg
// with the SQL before End — landing the statement in the span and the log.
//
// Only the four data processors are instrumented; gorm's noisy internal
// callbacks (row processing, transaction bookkeeping) are not observable as
// operations here, so no op-skip list is needed.
type observePlugin struct {
	system   string
	duration metric.Float64Histogram
}

// NewPlugin builds a gorm.Plugin that emits trace span + duration metric +
// access log for every operation under the given db.system label (e.g. "mysql",
// "postgresql", "clickhouse", "microsoft.sql_server").
func NewPlugin(system string) gorm.Plugin {
	return &observePlugin{system: system, duration: newDuration()}
}

func (p *observePlugin) Name() string { return "go-spring:observe" }

func (p *observePlugin) Initialize(db *gorm.DB) error {
	ops := []struct{ kind, raw string }{
		{"create", "gorm:create"},
		{"query", "gorm:query"},
		{"update", "gorm:update"},
		{"delete", "gorm:delete"},
	}
	for _, op := range ops {
		kind, raw := op.kind, op.raw
		before := func(tx *gorm.DB) {
			sp := p.start(tx.Statement.Context, kind)
			gormSpans.Store(tx, sp)
		}
		after := func(tx *gorm.DB) {
			v, ok := gormSpans.LoadAndDelete(tx)
			if !ok {
				return
			}
			sp := v.(*opSpan)
			if tx.Statement != nil {
				sp.SetArg(tx.Statement.SQL.String())
			}
			sp.End(tx.Error)
		}
		// gorm has no generic "register on every processor" API, so register per
		// kind against the gorm:<op> anchor each processor defines by default.
		switch kind {
		case "create":
			if err := db.Callback().Create().Before(raw).Register("go-spring:observe:before_create", before); err != nil {
				return err
			}
			if err := db.Callback().Create().After(raw).Register("go-spring:observe:after_create", after); err != nil {
				return err
			}
		case "query":
			if err := db.Callback().Query().Before(raw).Register("go-spring:observe:before_query", before); err != nil {
				return err
			}
			if err := db.Callback().Query().After(raw).Register("go-spring:observe:after_query", after); err != nil {
				return err
			}
		case "update":
			if err := db.Callback().Update().Before(raw).Register("go-spring:observe:before_update", before); err != nil {
				return err
			}
			if err := db.Callback().Update().After(raw).Register("go-spring:observe:after_update", after); err != nil {
				return err
			}
		case "delete":
			if err := db.Callback().Delete().Before(raw).Register("go-spring:observe:before_delete", before); err != nil {
				return err
			}
			if err := db.Callback().Delete().After(raw).Register("go-spring:observe:after_delete", after); err != nil {
				return err
			}
		}
	}
	return nil
}

// opSpan is one in-flight gorm operation: the client span opened in Before and
// ended in After.
type opSpan struct {
	p     *observePlugin
	ctx   context.Context
	span  trace.Span
	op    string
	arg   string
	start time.Time
}

// start opens the operation's client span.
func (p *observePlugin) start(ctx context.Context, op string) *opSpan {
	ctx, span := tracer.Start(ctx, op,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", p.system),
			attribute.String("db.operation", op),
		))
	return &opSpan{p: p, ctx: ctx, span: span, op: op, start: time.Now()}
}

// SetArg sets the SQL statement once it is known (in the After callback): it
// lands on the span as db.statement and in the access log, truncated so a
// runaway statement cannot flood either.
func (s *opSpan) SetArg(arg string) {
	s.arg = strutil.Truncate(arg, 512)
	if s.span != nil && s.arg != "" {
		s.span.SetAttributes(attribute.String("db.statement", s.arg))
	}
}

// End records the operation: the duration metric, the span (error status when
// err is set), and the access log — an error at Warn, a success with SQL at
// Debug (lazy, since the common case is uninteresting), other successes at
// Info.
func (s *opSpan) End(err error) {
	p := s.p
	dur := time.Since(s.start)
	status := "ok"
	if err != nil {
		status = "error"
	}
	p.duration.Record(s.ctx, dur.Seconds(), metric.WithAttributes(
		attribute.String("db.system", p.system),
		attribute.String("db.operation", s.op),
		attribute.String("status", status),
	))
	if err != nil {
		s.span.SetStatus(codes.Error, err.Error())
		s.span.RecordError(err)
	}
	s.span.End()

	common := []log.Field{
		log.String("system", p.system),
		log.String("operation", s.op),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	switch {
	case err != nil:
		log.Warn(s.ctx, accessTag, append(common, log.Any("error", err))...)
	case s.arg != "":
		fields := append(common, log.String("statement", s.arg))
		log.Debug(s.ctx, accessTag, func() []log.Field { return fields })
	default:
		log.Info(s.ctx, accessTag, common...)
	}
}
