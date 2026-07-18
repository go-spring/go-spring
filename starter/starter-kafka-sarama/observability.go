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

package StarterKafkaSarama

import (
	"context"

	"github.com/IBM/sarama"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Why these are call-site helpers rather than a wrapped producer/consumer:
//
//  1. The only official OTel instrumentation for sarama, otelsarama
//     (go.opentelemetry.io/contrib/instrumentation/github.com/Shopify/sarama/
//     otelsarama), is deprecated and still pinned to the abandoned
//     github.com/Shopify/sarama module. This starter uses github.com/IBM/sarama;
//     the two are distinct Go types, so otelsarama's WrapSyncProducer cannot wrap
//     an IBM producer, and importing it would drag in a second, conflicting
//     sarama fork. We therefore do the instrumentation natively on the OTel API.
//
//  2. sarama.SyncProducer.SendMessage takes no context.Context, so a producer
//     *wrapper* has nowhere to receive the request-scoped context from and could
//     only ever emit disconnected root spans. Passing ctx explicitly at the call
//     site is what makes distributed traces actually link across services.
//
// Everything here rides the OTel globals that starter-otel installs. Without
// starter-otel the global TracerProvider is a no-op and the global propagator is
// a no-op, so these helpers cost almost nothing and change no message bytes.

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-kafka-sarama"

// StartProducerSpan starts a producer span for msg and injects the current W3C
// trace context into msg.Headers so downstream consumers can continue the trace.
// Call it right before SyncProducer.SendMessage and end the returned span once
// the send completes, feeding any error back via the returned context's span:
//
//	ctx, span := StarterKafkaSarama.StartProducerSpan(ctx, msg)
//	_, _, err := producer.SendMessage(msg)
//	StarterKafkaSarama.EndSpan(span, err)
func StartProducerSpan(ctx context.Context, msg *sarama.ProducerMessage) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "kafka.produce "+msg.Topic,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", msg.Topic),
			attribute.String("messaging.operation", "publish"),
		),
	)
	otel.GetTextMapPropagator().Inject(ctx, producerCarrier{msg})
	return ctx, span
}

// StartConsumerSpan extracts the upstream trace context carried in msg.Headers
// and starts a consumer span as its child. Call it when a record is received and
// end the returned span once processing finishes:
//
//	ctx, span := StarterKafkaSarama.StartConsumerSpan(ctx, msg)
//	err := handle(ctx, msg)
//	StarterKafkaSarama.EndSpan(span, err)
func StartConsumerSpan(ctx context.Context, msg *sarama.ConsumerMessage) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, consumerCarrier{msg})
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "kafka.consume "+msg.Topic,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", msg.Topic),
			attribute.String("messaging.operation", "receive"),
			attribute.Int("messaging.kafka.partition", int(msg.Partition)),
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

// producerCarrier adapts a sarama.ProducerMessage's headers to the OTel
// TextMapCarrier interface for context injection.
type producerCarrier struct{ msg *sarama.ProducerMessage }

func (c producerCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c producerCarrier) Set(key, value string) {
	// Drop any existing header with the same key so re-injection stays idempotent.
	filtered := c.msg.Headers[:0]
	for _, h := range c.msg.Headers {
		if string(h.Key) != key {
			filtered = append(filtered, h)
		}
	}
	c.msg.Headers = append(filtered, sarama.RecordHeader{Key: []byte(key), Value: []byte(value)})
}

func (c producerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for _, h := range c.msg.Headers {
		keys = append(keys, string(h.Key))
	}
	return keys
}

// consumerCarrier adapts a sarama.ConsumerMessage's headers to the OTel
// TextMapCarrier interface for context extraction. Extraction never mutates the
// message, so Set is a no-op.
type consumerCarrier struct{ msg *sarama.ConsumerMessage }

func (c consumerCarrier) Get(key string) string {
	for _, h := range c.msg.Headers {
		if h != nil && string(h.Key) == key {
			return string(h.Value)
		}
	}
	return ""
}

func (c consumerCarrier) Set(string, string) {}

func (c consumerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.msg.Headers))
	for _, h := range c.msg.Headers {
		if h != nil {
			keys = append(keys, string(h.Key))
		}
	}
	return keys
}

var _ propagation.TextMapCarrier = producerCarrier{}
var _ propagation.TextMapCarrier = consumerCarrier{}
