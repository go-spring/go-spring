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

// observe.go declares what a mail send IS. The signals themselves — the span,
// the duration metrics, the access log — are emitted by the resilience layer,
// the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls reach an
// SMTP server.
//
// The vocabulary is its own: email is not messaging (there is no broker, no
// destination, no consumer side), so the labels are email.* rather than
// messaging.*. The result axis (status) is the ecosystem-wide one, so a mail
// failure still joins the same query shape as every other client.
package StarterMail

import (
	"go-spring.org/cloud/observability"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// emailSystem is the value the email.system label carries — the capability's
// name, not a per-instance choice.
const emailSystem = "smtp"

// accessTag is the static log tag for the mail access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("mail", "access")

// operation is the semantic identity of one send batch.
//
// The batch's size rides in Detail rather than Attrs: it varies per call, so as
// a metric label it would multiply the series. Detail reaches the span and the
// log and never a label, which also levels a send's success log at Debug (a
// send carries a count, so it almost always carries detail).
//
// The operation is declared non-idempotent: a resend is a second email, not a
// second attempt, so the executor chain must not retry it however a governance
// rule is configured.
func operation(msgs []*Message) observability.Operation {
	o := observability.Operation{
		Name:   "mail.send",
		Metric: "email.client",
		Attrs: []attribute.KeyValue{
			attribute.String("email.system", emailSystem),
			attribute.String("email.operation", "send"),
		},
		LogTag:        accessTag,
		NonIdempotent: true,
	}
	if d := sendDetail(msgs); len(d) > 0 {
		o.Detail = d
	}
	return o
}

// sendDetail renders the per-call detail the span and the log carry. It is
// deliberately not what the mail says or whom it is addressed to: recipients
// and subject are personal data, and on a failure the access log is written at
// Warn unconditionally — so the addresses would be recorded precisely when the
// send is already going wrong. The batch's size is what a span or a log line
// needs to be useful, and it is bounded, so it stays a detail rather than a
// label. The addresses stay in the caller's own records, where the data's owner
// put them. Bcc is not counted separately — the confidential list is not this
// layer's to see.
func sendDetail(msgs []*Message) []attribute.KeyValue {
	if len(msgs) == 0 {
		return nil
	}
	recipients := 0
	for _, m := range msgs {
		recipients += len(m.To) + len(m.Cc)
	}
	if recipients == 0 {
		return nil
	}
	return []attribute.KeyValue{
		attribute.Int("email.recipients.count", recipients),
	}
}
