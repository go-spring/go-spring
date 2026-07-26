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
// Mailer.Send and end the returned span once the send completes:
//
//	ctx, span := StarterMail.StartSendSpan(ctx, "notification")
//	err := mailer.Send(ctx, msg)
//	StarterMail.EndSpan(span, err)
//
// The span rides the OTel globals that starter-otel installs. Without
// starter-otel the global TracerProvider is a no-op. Importing starter-otel is
// the opt-in.
func StartSendSpan(ctx context.Context, purpose string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(tracerName).Start(ctx, "mail.send",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "smtp"),
			attribute.String("messaging.operation", "send"),
			attribute.String("mail.purpose", purpose),
		),
	)
	return ctx, span
}

// EndSpan records err (if any) on span and ends it. It is a small convenience so
// callers do not have to import the OTel codes package themselves.
func EndSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}
