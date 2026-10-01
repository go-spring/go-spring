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

// observe.go declares what a webhook delivery IS. The signals themselves — the
// span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls are an
// outbound notification to a chat-platform receiver.
//
// The vocabulary is the messaging family's: a delivery is a producer publish to
// a fixed external receiver (the span is a producer kind and the system is
// "webhook"), so messaging.system / messaging.operation describe it faithfully,
// and the metric prefix is messaging.client.
package StarterWebhook

import (
	"net/url"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// webhookSystem is the value the messaging.system label carries — the
// capability's name, not a per-instance choice.
const webhookSystem = "webhook"

// maxDetail bounds the destination host captured as per-call detail. A span or a
// log line has no use for more of it.
const maxDetail = 512

// accessTag is the static log tag for the webhook access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterAppTag].
var accessTag = log.RegisterAppTag("webhook", "access")

// operation is the semantic identity of one webhook delivery.
//
// messaging.system, messaging.operation and the channel ride in Attrs: all three
// are bounded (one system, one verb, five channels), so all three may label a
// metric without multiplying the series. The destination host rides in Detail
// instead: it is drawn from config and varies per instance, so as a metric label
// it would multiply the series — Detail reaches the span and the log, where the
// target is exactly what makes a line worth reading, and never a label. A
// delivery that resolves no host (an unparseable URL) carries no detail at all,
// which is also what levels its success log at Info.
func operation(channel, endpoint string) observability.Operation {
	o := observability.Operation{
		Name:   "webhook.send",
		Metric: "messaging.client",
		Attrs: []attribute.KeyValue{
			attribute.String("messaging.system", webhookSystem),
			attribute.String("messaging.operation", "send"),
			attribute.String("webhook.channel", channel),
		},
		LogTag: accessTag,
		// A delivery is the producer edge of the trace: the webhook endpoint is
		// downstream of us, and the edge is what keeps that visible once the
		// emitter opens the span.
		SpanKind: trace.SpanKindProducer,
	}
	if host := hostOf(endpoint); host != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("webhook.destination.host", strutil.Truncate(host, maxDetail)),
		}
	}
	return o
}

// hostOf extracts scheme://host for the span detail, keeping full endpoints —
// signed URLs with access tokens — out of telemetry.
func hostOf(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		return u.Scheme + "://" + u.Host
	}
	return ""
}
