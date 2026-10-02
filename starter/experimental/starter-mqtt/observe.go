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

// observe.go declares what an mqtt publish or consume IS. The signals
// themselves — the span, the duration metrics (call-level and attempt-level),
// the access log — are emitted by the resilience layer, the one place on the
// executor chain that sees a whole call. This file therefore holds no per-call
// emission code: only the vocabulary this starter alone knows, because only it
// knows these calls reach a broker.
//
// The one signal kept here is the connection-state counter: it is NOT a
// per-call signal — it is driven by the paho client's own connect / lost /
// reconnecting callbacks — so it is not the resilience layer's to emit and
// stays starter-local.
package StarterMQTT

import (
	"context"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// scope is the instrumentation scope name every meter and tracer in this package reports under.
const scope = "go-spring.org/starter-mqtt"

// accessTag is the static log tag for the mqtt access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("mqtt", "access")

// maxTopic bounds the topic captured as messaging.destination.name. A topic can
// be long and a span or a log line has no use for all of it.
const maxTopic = 512

// mqttSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const mqttSystem = "mqtt"

// Operation names, shared by the span name and the messaging.operation label so
// the two can never disagree.
const (
	opPublish = "publish"
	opConsume = "consume"
)

// Connection-state values the counter and the log lines share.
const (
	connConnected    = "connected"
	connLost         = "lost"
	connReconnecting = "reconnecting"
)

// operation is the semantic identity of one mqtt operation, named by its
// direction (publish / consume) and addressed by its topic.
//
// The direction and the system ride in Attrs — both bounded, so both may label a
// metric. The topic rides in Detail instead: topics are drawn from an open set,
// so as a metric label one would multiply the series without bound. Detail
// reaches the span and the log — where a topic is exactly what makes a line
// worth reading — and never a label. A topicless call carries no detail at all,
// which is also what levelled its success log at Info.
// spanKind maps the operation's direction onto the trace edge it adds: a publish
// is the producer side of the link, a consume the consumer side.
func spanKind(direction string) trace.SpanKind {
	if direction == opConsume {
		return trace.SpanKindConsumer
	}
	return trace.SpanKindProducer
}

func operation(direction, topic string) observability.Operation {
	op := observability.Operation{
		Name:   direction,
		Metric: "messaging.client",
		Attrs: []attribute.KeyValue{
			attribute.String("messaging.system", mqttSystem),
			attribute.String("messaging.operation", direction),
		},
		LogTag: accessTag,
		// A publish is the producer edge of the trace and a consume its consumer edge;
		// declaring the kind is what keeps that topology once the emitter opens the span.
		SpanKind: spanKind(direction),
		// Publishing and handling both repeat a side effect when retried — a second
		// message delivered, or a second run of the handler — so the executor chain must
		// not retry this operation whatever retry a governance rule asks for.
		NonIdempotent: true,
	}
	if topic != "" {
		op.Detail = []attribute.KeyValue{
			attribute.String("messaging.destination.name", strutil.Truncate(topic, maxTopic)),
		}
	}
	return op
}

// connStateCounter counts the connection-lifecycle transitions paho reports.
// Those events had only log lines: a client that kept losing its broker was
// visible in text and invisible to every dashboard — and they are the signal
// that precedes publishes starting to fail.
//
// The driver builds it, next to the callbacks it counts for, because paho takes
// the handlers only as construction options: there is no post-construction slot
// to install them from the starter's side, so splitting the pair would mean
// dropping one of them.
type connStateCounter struct {
	changes metric.Int64Counter
}

// newConnStateCounter builds the counter from whatever meter provider is
// current — invoked at wiring time (inside the driver), not at package init, so
// an SDK installed later still receives the records.
func newConnStateCounter() *connStateCounter {
	changes, _ := otel.Meter(scope).Int64Counter("messaging.client.connection.state_changes",
		metric.WithDescription("Connection-state transitions reported by the messaging client"),
		metric.WithUnit("{event}"))
	return &connStateCounter{changes: changes}
}

// record counts one transition and returns the fields its log line carries.
// Returning them is what keeps the metric's attribute and the log's key from
// being spelled twice — the drift this pairing exists to prevent, and a drifted
// key is silent: the line still looks right and joins nothing.
func (c *connStateCounter) record(ctx context.Context, state string) []log.Field {
	c.changes.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", "mqtt"),
		attribute.String("state", state),
	))
	return []log.Field{
		log.String("messaging.system", "mqtt"),
		log.String("state", state),
	}
}
