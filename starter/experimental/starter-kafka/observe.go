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

// observe.go declares what a kafka publish or consume IS. The signals
// themselves — the span, the duration metrics (call-level and attempt-level),
// the access log — are emitted by the resilience layer, the one place on the
// executor chain that sees a whole call. This file therefore holds no emission
// code: only the vocabulary this starter alone knows, because only it knows
// these calls reach a broker.
//
// What is NOT declared here is the broker/client-level telemetry kotel installs
// on the franz-go client (messaging.kafka.* connections, bytes, per-node
// health). That is third-party instrumentation this starter merely enables, not
// a per-call signal, so it stays where kotel puts it.
package StarterKafka

import (
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// kafkaSystem is the value the family's messaging.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const kafkaSystem = "kafka"

// accessTag is the static log tag for the kafka access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("messaging", "access")

// maxDestination bounds the topic captured as messaging.destination.name. A
// topic can be long and a span or a log line has no use for all of it.
const maxDestination = 512

// Operation names, shared by the span name and the messaging.operation label so
// the two can never disagree.
const (
	opPublish = "publish"
	opConsume = "consume"
)

// operation is the semantic identity of one kafka operation, named by its
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
			attribute.String("messaging.system", kafkaSystem),
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
			attribute.String("messaging.destination.name", strutil.Truncate(topic, maxDestination)),
		}
	}
	return op
}
