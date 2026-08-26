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

// command.go is the "command seam" concept of this starter: the call-site
// observe helpers (trace span + producer/consumer metrics-and-log) and the
// resilience guard (GuardedPublish). amqp091-go exposes no reject-capable
// middleware and channels are caller-created from the raw *amqp.Connection bean,
// so both layers are opt-in call-site helpers rather than a transparent
// interceptor — the analog of starter-go-redis's command.go, only without a
// native chain to fold onto.
package StarterRabbitMQ

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	resilobserve "go-spring.org/cloud/observe/resilience"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Why these are call-site helpers rather than a wrapped channel/publisher:
//
//  1. amqp091-go has no official OTel instrumentation, and this starter's bean
//     is an *amqp.Connection — channels, publishes and deliveries are all
//     created by the caller, so the starter has no seam to auto-instrument. A
//     wrapper would have to re-expose the whole Channel surface (Publish,
//     Consume, Get, Ack, Qos, ExchangeDeclare, ...) and would still miss anything
//     the caller does on the raw connection.
//
//  2. amqp.Publishing carries a Headers table (amqp.Table) and every delivery
//     echoes it back, so W3C trace context propagates cleanly across the broker.
//     Instrumenting at the call site — right where the caller already holds the
//     Publishing / Delivery — is what makes distributed traces link producer to
//     consumer, which a connection-level wrapper cannot do.
//
// Everything here rides the OTel globals that starter-otel installs. Without
// starter-otel the global TracerProvider and propagator are no-ops, so these
// helpers cost almost nothing and change no message bytes.

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-rabbitmq"

// StartPublishSpan starts a producer span for a publish to exchange/routingKey
// and injects the current W3C trace context into pub.Headers so consumers can
// continue the trace. Call it right before Channel.PublishWithContext and end
// the returned span once the publish returns:
//
//	ctx, span := StarterRabbitMQ.StartPublishSpan(ctx, exchange, key, &pub)
//	err := ch.PublishWithContext(ctx, exchange, key, false, false, pub)
//	StarterRabbitMQ.EndSpan(span, err)
func StartPublishSpan(ctx context.Context, exchange, routingKey string, pub *amqp.Publishing) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	dest := exchange
	if dest == "" {
		// The default exchange routes by queue name carried in the routing key.
		dest = routingKey
	}
	ctx, span := tracer.Start(ctx, "rabbitmq.publish "+dest,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", exchange),
			attribute.String("messaging.rabbitmq.destination.routing_key", routingKey),
			attribute.String("messaging.operation", "publish"),
		),
	)
	if pub.Headers == nil {
		pub.Headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, publishingCarrier{pub})
	return ctx, span
}

// StartConsumeSpan extracts the upstream trace context carried in the delivery's
// headers and starts a consumer span as its child. Call it when a delivery is
// received and end the returned span once processing finishes:
//
//	ctx, span := StarterRabbitMQ.StartConsumeSpan(ctx, &delivery)
//	err := handle(ctx, delivery)
//	StarterRabbitMQ.EndSpan(span, err)
func StartConsumeSpan(ctx context.Context, d *amqp.Delivery) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, deliveryCarrier{d})
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	dest := d.Exchange
	if dest == "" {
		dest = d.RoutingKey
	}
	ctx, span := tracer.Start(ctx, "rabbitmq.consume "+dest,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "rabbitmq"),
			attribute.String("messaging.destination.name", d.Exchange),
			attribute.String("messaging.rabbitmq.destination.routing_key", d.RoutingKey),
			attribute.String("messaging.operation", "receive"),
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

// publishingCarrier adapts an amqp.Publishing's Headers table to the OTel
// TextMapCarrier interface for context injection.
type publishingCarrier struct{ pub *amqp.Publishing }

func (c publishingCarrier) Get(key string) string {
	if v, ok := c.pub.Headers[key].(string); ok {
		return v
	}
	return ""
}

func (c publishingCarrier) Set(key, value string) {
	c.pub.Headers[key] = value
}

func (c publishingCarrier) Keys() []string {
	keys := make([]string, 0, len(c.pub.Headers))
	for k := range c.pub.Headers {
		keys = append(keys, k)
	}
	return keys
}

// deliveryCarrier adapts an amqp.Delivery's Headers table to the OTel
// TextMapCarrier interface for context extraction. Extraction never mutates the
// delivery, so Set is a no-op.
type deliveryCarrier struct{ d *amqp.Delivery }

func (c deliveryCarrier) Get(key string) string {
	if v, ok := c.d.Headers[key].(string); ok {
		return v
	}
	return ""
}

func (c deliveryCarrier) Set(string, string) {}

func (c deliveryCarrier) Keys() []string {
	keys := make([]string, 0, len(c.d.Headers))
	for k := range c.d.Headers {
		keys = append(keys, k)
	}
	return keys
}

var _ propagation.TextMapCarrier = publishingCarrier{}
var _ propagation.TextMapCarrier = deliveryCarrier{}

// --- binder auto path (kit-backed) -------------------------------------------
//
// The messaging.Binder drives publish/consume through the observe kit (3 signals)
// via these package-level observers. They use a default "brief" level; the bean
// is a raw *amqp.Connection (no wrapper to carry per-instance config), so the
// binder path cannot bind a per-instance ObserveConfig — the manual helpers above
// remain for apps that want explicit control.

var (
	defaultPubObs = observe.NewProducer("rabbitmq", observe.ObserveConfig{Level: observe.DefaultBrief})
	defaultSubObs = observe.NewConsumer("rabbitmq", observe.ObserveConfig{Level: observe.DefaultBrief})
)

// startPublish opens a producer observation and injects the W3C trace context
// into pub.Headers. For the binder's publish path.
func startPublish(ctx context.Context, routingKey string, pub *amqp.Publishing) (context.Context, *observe.Span) {
	ctx, sp := defaultPubObs.Start(ctx, "publish", routingKey)
	if pub.Headers == nil {
		pub.Headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, publishingCarrier{pub})
	return ctx, sp
}

// startConsume extracts the upstream trace from the delivery and opens a consumer
// observation. For the binder's consume loop.
func startConsume(ctx context.Context, d *amqp.Delivery) (context.Context, *observe.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, deliveryCarrier{d})
	dest := d.Exchange
	if dest == "" {
		dest = d.RoutingKey
	}
	return defaultSubObs.Start(ctx, "consume", dest)
}

// resilienceExecs tracks the resilience executor attached to each connection,
// so GuardedPublish can resolve it from the raw *amqp.Connection bean and the
// destructor can Close it (releasing any background resources of a production
// driver). Only connections with resilience enabled appear here.
var resilienceExecs sync.Map // *amqp.Connection -> resilience.Executor

// resilienceResources tracks the stable resource label per connection so the
// guard can pass it to exec.Execute without re-deriving from Config.
var resilienceResources sync.Map // *amqp.Connection -> string

// applyResilience builds an executor and indexes it by conn. This is the
// rabbitmq seam of resilience: amqp091 exposes no reject-capable middleware and
// channels are caller-created from the connection, so the executor is driven
// through an opt-in call-site guard (GuardedPublish) rather than a transparent
// interceptor.
//
// The executor is resolved through the neutral [resilience.ExecutorFor] seam,
// which starter-govern backs with the governance center — so this function has
// zero coupling to cloud/governance. When governance is off, ExecutorFor yields a
// transparent no-op executor; fault wraps it when enabled.
func applyResilience(c Config, conn *amqp.Connection, resource string) error {
	exec := fault.WrapExecutor(resilience.ExecutorFor(resource))
	exec = resilobserve.WrapExecutor(exec, "rabbitmq", c.Observability)
	resilienceExecs.Store(conn, exec)
	resilienceResources.Store(conn, resource)
	return nil
}

// closeResilience closes and forgets the executor behind conn, if any.
func closeResilience(conn *amqp.Connection) {
	if v, ok := resilienceExecs.LoadAndDelete(conn); ok {
		_ = v.(resilience.Executor).Close()
	}
	resilienceResources.Delete(conn)
}

// guard routes call through the executor attached to conn, and otherwise runs it
// inline. When resilience is disabled for the connection this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, conn *amqp.Connection, call func(context.Context) error) error {
	v, ok := resilienceExecs.Load(conn)
	if !ok {
		return call(ctx)
	}
	r, _ := resilienceResources.Load(conn)
	return v.(resilience.Executor).Execute(ctx, r.(string), call)
}

// GuardedPublish publishes pub to exchange/routingKey on ch, routed through the
// resilience executor attached to conn when governance is enabled.
// When governance is disabled this behaves exactly like ch.PublishWithContext.
// On rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying publish is never invoked.
//
// The connection (not the channel) is passed to resolve the executor because a
// channel may outlive the connection bean in some patterns, while the executor
// is always scoped to the connection the starter created.
func GuardedPublish(ctx context.Context, conn *amqp.Connection, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	return guard(ctx, conn, func(ctx context.Context) error {
		return ch.PublishWithContext(ctx, exchange, key, mandatory, immediate, pub)
	})
}
