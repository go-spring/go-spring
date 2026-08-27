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

// Package transactionobserve is the shared distributed-transaction
// instrumentation adapter for the go-spring observability story. It provides
// [transaction.Observer] implementations (one named type per model, sharing one
// Begin) that open one OTel child span per transaction phase, so the saga/tcc/at
// starters share one implementation instead of copy-pasting an otelObserver
// each.
//
// It lives in the observe package (rather than inside the otel-free
// abstraction packages cloud/experimental/transaction that define the Observer
// interfaces, or copy-pasted per starter): N starters, one shared adapter.
// Everything rides the otel globals starter-otel installs; without it the
// global tracer is a no-op, so an unconfigured app pays almost nothing.
//
// A starter installs the matching observer at coordinator construction:
//
//	opts = append(opts, transaction.WithObserver(transactionobserve.SagaObserver{}))
package transactionobserve

import (
	"context"

	"go-spring.org/cloud/experimental/transaction"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this adapter, following the convention
// used by the other shared observe adapters.
const tracerName = "go-spring.org/cloud/observe/transaction"

// beginSpan opens a span named name as a child of ctx, tagged with attrs, and
// returns an end func that records err on the span (setting error status) and
// ends it. Shared by the saga/tcc/at observers below.
func beginSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// SagaObserver implements [transaction.Observer] for the Saga model by opening
// one child span per step phase: "saga.action <step>" or
// "saga.compensate <step>", tagged with the saga id, step name and phase.
type SagaObserver struct{}

var _ transaction.Observer = SagaObserver{}

// Begin starts a span for one saga step phase. The returned function ends the
// span, recording the error when the phase failed so a compensated (or failed)
// saga is visible in the trace.
func (SagaObserver) Begin(ctx context.Context, txID, step string, phase transaction.Phase) (context.Context, func(error)) {
	return beginSpan(ctx, sagaSpanName(phase, step),
		attribute.String("saga.id", txID),
		attribute.String("saga.step", step),
		attribute.String("saga.phase", phase.String()),
	)
}

func sagaSpanName(phase transaction.Phase, step string) string {
	if phase == transaction.PhaseCompensate {
		return "saga.compensate " + step
	}
	return "saga.action " + step
}

// TccObserver implements [transaction.Observer] for the TCC model (it is what
// [tcc.Observer] aliases) by opening one child span per participant phase:
// "tcc.try/confirm/cancel <participant>", tagged with the transaction id,
// participant name and phase.
type TccObserver struct{}

var _ transaction.Observer = TccObserver{}

// Begin starts a span for one participant phase. The returned function ends the
// span, recording the error when the phase failed so a cancelled (or failed)
// transaction is visible in the trace.
func (TccObserver) Begin(ctx context.Context, txID, participant string, phase transaction.Phase) (context.Context, func(error)) {
	return beginSpan(ctx, tccSpanName(phase, participant),
		attribute.String("tcc.id", txID),
		attribute.String("tcc.participant", participant),
		attribute.String("tcc.phase", phase.String()),
	)
}

func tccSpanName(phase transaction.Phase, participant string) string {
	switch phase {
	case transaction.PhaseConfirm:
		return "tcc.confirm " + participant
	case transaction.PhaseCancel:
		return "tcc.cancel " + participant
	default:
		return "tcc.try " + participant
	}
}

// AtObserver implements [transaction.Observer] for the AT model (it is what
// [at.Observer] aliases) by opening one child span per branch second-phase
// operation: "at.commit <branch>" or "at.rollback <branch>", tagged with the
// global transaction id, branch id and phase.
type AtObserver struct{}

var _ transaction.Observer = AtObserver{}

// Begin starts a span for one branch phase. The returned function ends the
// span, recording the error when the phase failed so a failed commit/rollback
// is visible in the trace.
func (AtObserver) Begin(ctx context.Context, txID, branch string, phase transaction.Phase) (context.Context, func(error)) {
	return beginSpan(ctx, atSpanName(phase, branch),
		attribute.String("at.xid", txID),
		attribute.String("at.branch", branch),
		attribute.String("at.phase", phase.String()),
	)
}

func atSpanName(phase transaction.Phase, branch string) string {
	if phase == transaction.PhaseRollback {
		return "at.rollback " + branch
	}
	return "at.commit " + branch
}
