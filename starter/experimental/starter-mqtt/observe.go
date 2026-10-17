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

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-mqtt"

// accessTag is the static log tag for the mqtt access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("mqtt", "access")

// mqttSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const mqttSystem = "mqtt"

// opWord is one messaging operation this starter reports: the OTel operation
// word — carried by `messaging.operation`, the span name and the access log —
// paired with the interaction shape it projects onto, which picks the trace
// edge. Stating the two together keeps them from drifting, and makes a
// misspelled word a COMPILE error instead of a wrong metric label: the shape is
// the one that word means, not a fallback for words nobody recognized.
type opWord struct {
	word  string
	shape chain.Kind
}

var (
	// opSend is a one-way hand-off to the broker: the producer edge.
	opSend = opWord{word: "send", shape: chain.KindPublish}
	// opProcess is the handling of one delivered message: the consumer edge.
	opProcess = opWord{word: "process", shape: chain.KindConsume}
)

// Connection-state values the counter and the log lines share.
const (
	connConnected    = "connected"
	connLost         = "lost"
	connReconnecting = "reconnecting"
)

// operation is the semantic identity of one mqtt operation: the operation
// word it reports and the destination it addresses.
//
// The vocabulary — the family's attribute keys, the bounded/open split that puts
// the destination in detail and the system in a label, and the trace edge each
// word adds — is the messaging family's, not this file's; it lives in
// [cloud/chain]. This starter supplies only what only it knows: that the calls
// reach an mqtt broker, and their word and destination.
func operation(word opWord, topic string) chain.Operation {
	op := chain.MessagingOp(word.shape, mqttSystem, word.word, topic)
	op.Observe.Log.Tag = accessTag
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
	changes, _ := otel.Meter(componentName).Int64Counter("messaging.client.connection.state_changes",
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
