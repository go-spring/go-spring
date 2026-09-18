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

package StarterMail

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-mail"

// StartSendSpan starts a client span for an SMTP send. Call it before
// Mailer.Send and end the returned span once the send completes. It is optional:
// Mailer.Send records the metric and the access log on its own, so a send is
// measured either way — this only adds the trace and the caller-supplied purpose.
//
//	ctx, span := StarterMail.StartSendSpan(ctx, "notification")
//	err := mailer.Send(ctx, msg)
//	StarterMail.EndSpan(span, err)
//
// The span rides the OTel globals that starter-otel installs. Without
// starter-otel the global TracerProvider is a no-op. Importing starter-otel is
// the opt-in.
//
// The attribute names are email.*, not messaging.*: email is not messaging —
// there is no broker, no destination and no consumer side. The metric and the
// access log in observe.go use the same email.system key, so the three signals
// join.
func StartSendSpan(ctx context.Context, purpose string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "mail.send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("email.system", emailSystem),
			attribute.String("email.operation", "send"),
			attribute.String("mail.purpose", purpose),
		),
	)
	return ctx, span
}

// EndSpan records the outcome on span and ends it. It is a small convenience so
// callers do not have to import the OTel codes package themselves.
//
// The status attribute carries the ecosystem-wide result axis (ok|error) — the
// same two words the metric label and the log field use — while the gRPC-style
// error detail rides the span status and the recorded error.
func EndSpan(span trace.Span, err error) {
	span.SetAttributes(attribute.String("status", statusOf(err)))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
