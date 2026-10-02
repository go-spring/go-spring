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

// observe.go declares what a NATS publish or consume IS. The signals
// themselves — the span, the duration metrics (call-level and attempt-level),
// the access log — are emitted by the resilience layer, the one place on the
// executor chain that sees a whole call. This file therefore holds no per-call
// emission code: only the vocabulary this starter alone knows, because only it
// knows these calls reach a broker.
//
// The one signal kept here is the connection-state counter: it is NOT a
// per-call signal — it is driven by the NATS client's own disconnect /
// reconnect / close callbacks — so it is not the resilience layer's to emit and
// stays starter-local.
package StarterNats

import (
	"context"
	"sync"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// accessTag is the static log tag for the nats access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("nats", "access")

// scope is the instrumentation scope name every meter and tracer in this package
// reports under.
const scope = "go-spring.org/starter-nats"

// maxSubject bounds the subject captured as messaging.destination.name. A
// subject can be long and a span or a log line has no use for all of it.
const maxSubject = 512

// natsSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const natsSystem = "nats"

// Operation names, shared by the span name and the messaging.operation label so
// the two can never disagree.
const (
	opPublish = "publish"
	opConsume = "consume"
)

// Connection-state values the counter and the log lines share.
const (
	connDisconnected = "disconnected"
	connReconnected  = "reconnected"
	connClosed       = "closed"
)

// operation is the semantic identity of one NATS operation, named by its
// direction (publish / consume) and addressed by its subject.
//
// The direction and the system ride in Attrs — both bounded, so both may label a
// metric. The subject rides in Detail instead: subjects are drawn from an open
// set, so as a metric label one would multiply the series without bound. Detail
// reaches the span and the log — where a subject is exactly what makes a line
// worth reading — and never a label. A subjectless call carries no detail at
// all, which is also what levelled its success log at Info.
// spanKind maps the operation's direction onto the trace edge it adds: a publish
// is the producer side of the link, a consume the consumer side.
func spanKind(direction string) trace.SpanKind {
	if direction == opConsume {
		return trace.SpanKindConsumer
	}
	return trace.SpanKindProducer
}

func operation(direction, subject string) observability.Operation {
	op := observability.Operation{
		Name:   direction,
		Metric: "messaging.client",
		Attrs: []attribute.KeyValue{
			attribute.String("messaging.system", natsSystem),
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
	if subject != "" {
		op.Detail = []attribute.KeyValue{
			attribute.String("messaging.destination.name", strutil.Truncate(subject, maxSubject)),
		}
	}
	return op
}

// instrumentSet is this starter's instrument set: one per process, resolved
// lazily on first use so it binds to whichever providers are current then, and
// shared by every connection in the process. It holds only the connection-state
// counter; the per-call signals are the resilience layer's, built from the
// operation this starter declares (see [operation]).
type instrumentSet struct {
	connChanges metric.Int64Counter
}

// instruments is the one instrument set this starter uses for the whole
// process. Resolution is deferred to the first use, not run at package init, so
// the instruments bind to whichever providers are current then - starter-otel
// installs them before any bean is built, but a test may replace them later and
// a value resolved at init would keep pointing at the old SDK.
var instruments = sync.OnceValue(buildInstruments)

// resetInstruments makes the next use of instruments() resolve a fresh set. It
// exists for tests that install their own MeterProvider: the set is process-wide
// and resolved once, so a test running after one that already resolved it would
// otherwise keep reporting into the earlier provider.
func resetInstruments() { instruments = sync.OnceValue(buildInstruments) }

func buildInstruments() *instrumentSet {
	m := otel.Meter(scope)
	in := &instrumentSet{}
	in.connChanges, _ = m.Int64Counter("messaging.client.connection.state_changes",
		metric.WithDescription("Connection-state transitions reported by the messaging client"),
		metric.WithUnit("{event}"))
	return in
}

// connStateCounter counts the connection-state transitions the NATS client's own
// handlers report. Those events had only log lines: a connection that flapped or
// died was visible in text and invisible to every dashboard — and they are the
// signal that precedes operations starting to fail.
//
// The driver builds it, next to the handlers it counts for, because nats.Conn's
// Set*Handler methods REPLACE a slot rather than chaining: a counter installed
// from the starter's lifecycle would silently displace whatever handlers a
// custom Driver had set. Keeping the pair in one place is what lets both survive.
type connStateCounter struct {
	ins *instrumentSet
}

// newConnStateCounter takes the shared instrument set — invoked at wiring time
// (inside the driver), not at package init, so an SDK installed later still
// receives the records.
func newConnStateCounter() *connStateCounter {
	return &connStateCounter{ins: instruments()}
}

// record counts one transition and returns the fields its log line carries.
// Returning them is what keeps the metric's attribute and the log's key from
// being spelled twice — the drift this pairing exists to prevent, and a drifted
// key is silent: the line still looks right and joins nothing.
func (c *connStateCounter) record(ctx context.Context, state string) []log.Field {
	c.ins.connChanges.Add(ctx, 1, metric.WithAttributes(
		attribute.String("messaging.system", natsSystem),
		attribute.String("state", state),
	))
	return []log.Field{
		log.String("messaging.system", natsSystem),
		log.String("state", state),
	}
}
