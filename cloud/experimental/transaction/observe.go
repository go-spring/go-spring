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

package transaction

import (
	"context"
	"sync"

	"go-spring.org/cloud/observability/instrument"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// observerTracerName identifies spans emitted by the observers below.
const observerTracerName = "go-spring.org/cloud/experimental/transaction"

// BeginSpan opens the standard OTel span for one transaction phase. Span
// handling is the STANDARD OTel capability, not part of an observability seam:
// the core packages call it directly. The returned func records err on the span
// (setting error status) and ends it.
func BeginSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, func(error)) {
	ctx, span := otel.GetTracerProvider().Tracer(observerTracerName).Start(ctx, name, trace.WithAttributes(attrs...))
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

// BeginSagaSpan opens the span for one saga step phase: "saga.action <step>" or
// "saga.compensate <step>", tagged with the saga id, step name and phase.
func BeginSagaSpan(ctx context.Context, txID, step string, phase Phase) (context.Context, func(error)) {
	return BeginSpan(ctx, sagaSpanName(phase, step),
		attribute.String("saga.id", txID),
		attribute.String("saga.step", step),
		attribute.String("saga.phase", phase.String()),
	)
}

func sagaSpanName(phase Phase, step string) string {
	if phase == PhaseCompensate {
		return "saga.compensate " + step
	}
	return "saga.action " + step
}

// BeginTccSpan opens the span for one TCC participant phase:
// "tcc.try/confirm/cancel <participant>", tagged with the transaction id,
// participant name and phase.
func BeginTccSpan(ctx context.Context, txID, participant string, phase Phase) (context.Context, func(error)) {
	return BeginSpan(ctx, tccSpanName(phase, participant),
		attribute.String("tcc.id", txID),
		attribute.String("tcc.participant", participant),
		attribute.String("tcc.phase", phase.String()),
	)
}

func tccSpanName(phase Phase, participant string) string {
	switch phase {
	case PhaseConfirm:
		return "tcc.confirm " + participant
	case PhaseCancel:
		return "tcc.cancel " + participant
	default:
		return "tcc.try " + participant
	}
}

// BeginAtSpan opens the span for one AT branch second-phase operation:
// "at.commit <branch>" or "at.rollback <branch>", tagged with the global
// transaction id, branch id and phase.
func BeginAtSpan(ctx context.Context, txID, branch string, phase Phase) (context.Context, func(error)) {
	return BeginSpan(ctx, atSpanName(phase, branch),
		attribute.String("at.xid", txID),
		attribute.String("at.branch", branch),
		attribute.String("at.phase", phase.String()),
	)
}

func atSpanName(phase Phase, branch string) string {
	if phase == PhaseRollback {
		return "at.rollback " + branch
	}
	return "at.commit " + branch
}

// outcomeMeta names each model's terminal-outcome counter: the metric scope,
// description and unit it had before the observers were separated, kept
// byte-for-byte so dashboards continue to read it.
var outcomeMeta = map[string]struct {
	scope string
	desc  string
	unit  string
}{
	"saga": {
		scope: componentName,
		desc:  "Saga terminal states, by status",
		unit:  "{saga}",
	},
	"tcc": {
		scope: "go-spring.org/cloud/experimental/transaction/tcc",
		desc:  "TCC terminal states, by status",
		unit:  "{transaction}",
	},
}

// outcomeObserver is the default [MetricsObserver]: one counter per model,
// resolved lazily on first outcome so it binds to whichever OTel provider is
// current then.
type outcomeObserver struct {
	mu       sync.Mutex
	counters map[string]metric.Int64Counter
}

// Outcome implements [MetricsObserver].
func (o *outcomeObserver) Outcome(ctx context.Context, model, status string) {
	o.mu.Lock()
	counter, ok := o.counters[model]
	if !ok {
		meta, named := outcomeMeta[model]
		if !named {
			o.mu.Unlock()
			return
		}
		var err error
		counter, err = otel.Meter(meta.scope).Int64Counter("transaction."+model+".outcome.total",
			metric.WithDescription(meta.desc),
			metric.WithUnit(meta.unit))
		if err != nil {
			reportOutcomeInstrumentError(model, err)
		}
		o.counters[model] = counter
	}
	o.mu.Unlock()
	if counter != nil {
		counter.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	}
}

// NewMetricsOTelObserver returns the default OTel-backed [MetricsObserver].
func NewMetricsOTelObserver() (MetricsObserver, error) {
	return &outcomeObserver{counters: map[string]metric.Int64Counter{}}, nil
}

// reportOutcomeInstrumentError surfaces a failed outcome-counter registration;
// the model's metric degrades to a no-op rather than dropping the transaction.
func reportOutcomeInstrumentError(model string, err error) {
	instrument.ReportInstrumentError("transaction."+model, err)
}
