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

package StarterTransactionATGorm

import (
	"context"

	"go-spring.org/spring/transaction/at"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-transaction-at-gorm"

// otelObserver implements [at.Observer] by opening one child span per branch
// second-phase operation. This is the starter-side contribution that fulfils the
// design's observability requirement without pulling otel into stdlib: the
// coordinator calls Begin around every branch commit and rollback, and the
// returned end func records the outcome on the span. Everything rides the otel
// globals starter-otel installs; without it the global tracer is a no-op, so an
// unconfigured app pays almost nothing.
type otelObserver struct{}

var _ at.Observer = otelObserver{}

// Begin starts a span named "at.<phase> <branch>" as a child of ctx, tagged with
// the global transaction id, branch id and phase. The returned function ends the
// span, recording the error when the phase failed so a failed commit/rollback is
// visible in the trace.
func (otelObserver) Begin(ctx context.Context, xid, branch string, phase at.Phase) (context.Context, func(error)) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, spanName(phase, branch),
		trace.WithAttributes(
			attribute.String("at.xid", xid),
			attribute.String("at.branch", branch),
			attribute.String("at.phase", phase.String()),
		),
	)
	return ctx, func(err error) {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

func spanName(phase at.Phase, branch string) string {
	if phase == at.PhaseRollback {
		return "at.rollback " + branch
	}
	return "at.commit " + branch
}
