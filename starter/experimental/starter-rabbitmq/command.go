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

// command.go is the "command seam" concept of this starter: the per-operation
// declaration and protection that wraps publishes and consumes. Two concerns
// live here:
//
//	declare    — GuardedPublish (and the driver's consume path in client.go)
//	             declare the operation's identity (see [operation]) on the ctx
//	             and inject/extract the W3C trace context across the broker.
//	resilience — the InnerPublisher chain (see entity.go) runs every publish
//	             under the connection-scoped executor, since amqp091 exposes
//	             no reject-capable middleware. The executor is also the single
//	             emitter: it reads the declared operation off the ctx and
//	             opens the span, records the durations (call-level and
//	             attempt-level) and writes the one access log.
package StarterRabbitMQ

import (
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// Why the resilience guard is a call-site helper rather than a wrapped channel:
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
//     Injecting at the call site — right where the caller already holds the
//     Publishing / Delivery — is what makes distributed traces link producer to
//     consumer, which a connection-level wrapper cannot do.
//
// Everything here rides the OTel globals that starter-otel installs. Without
// starter-otel the global propagator is a no-op, so these helpers cost almost
// nothing and change no message bytes.

// injectW3C inserts the current trace context into pub.Headers so the receiver
// can continue the trace across the broker. With no valid span on ctx (the
// ungoverned path) it writes nothing.
func injectW3C(ctx context.Context, pub *amqp.Publishing) {
	if pub.Headers == nil {
		pub.Headers = amqp.Table{}
	}
	otel.GetTextMapPropagator().Inject(ctx, publishingCarrier{pub})
}

// extractW3C pulls the upstream trace context out of d.Headers. Returns the
// input ctx unchanged when no context was propagated.
func extractW3C(ctx context.Context, d *amqp.Delivery) context.Context {
	if len(d.Headers) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, deliveryCarrier{d})
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

// InnerPublisher is the seam a connection's publishes run through. amqp091
// exposes no reject-capable middleware and delivers a concrete connection, so
// this interface is the ONLY way to modify what happens under the promoted
// Publish.
//
// The default chain is the identity layer over the governance layer over a raw
// adapter, and the embedded InnerPublisher is where a custom layer goes:
// implement this interface (embed the head you found to inherit the methods you
// do not care about), then assign your layer over it. The chain under the layer
// keeps doing its job — the destinations a layer rewrites are what the identity
// layer declares, and the executor still protects every publish.
//
// Release follows the chain protocol; the connection's own lifecycle belongs to
// [Client.Close], so the raw layer's Release is a pass-through at either depth.
type InnerPublisher interface {
	// Publish publishes pub to exchange/routingKey on ch (a caller-created
	// channel; channels are not safe for concurrent use).
	Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawPublisher is the adapter layer: it injects the W3C trace context into the
// publishing (from the attempt ctx the layers above handed down, so the
// traceparent carries the executor's span) and makes the wire call.
type RawPublisher struct{}

// NewRawPublisher builds the adapter layer.
func NewRawPublisher() *RawPublisher { return &RawPublisher{} }

// Release is the protocol's pass-through: the connection's lifecycle belongs
// to [Client.Close], not the chain.
func (r *RawPublisher) Release(bool) error { return nil }

func (r *RawPublisher) Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	injectW3C(ctx, &pub)
	return ch.PublishWithContext(ctx, exchange, key, mandatory, immediate, pub)
}

// GuardPublisher is the governance layer: it runs every publish under the
// resilience executor, which applies rate limiting, breaking and fault
// injection — and emits the publish's span, metrics and access log from the one
// point that sees the whole call, attempts included. [NewGuardPublisher]
// builds it.
type GuardPublisher struct {
	exec chain.Executor
	next InnerPublisher
}

// NewGuardPublisher builds the governance layer over next, running every
// publish under exec.
func NewGuardPublisher(next InnerPublisher, exec chain.Executor) *GuardPublisher {
	return &GuardPublisher{exec: exec, next: next}
}

// Release hands releaseRaw to the layer under it — the executor is closed by
// [Client.Close], with the connection it is scoped to.
func (g *GuardPublisher) Release(releaseRaw bool) error { return g.next.Release(releaseRaw) }

func (g *GuardPublisher) Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	if g.exec == nil {
		return g.next.Publish(ctx, ch, exchange, key, mandatory, immediate, pub)
	}
	return g.exec.Execute(ctx, func(attemptCtx context.Context) error {
		return g.next.Publish(attemptCtx, ch, exchange, key, mandatory, immediate, pub)
	})
}

// ObsPublisher is the identity layer at the head: it names the publish — with
// the destination (the exchange, or the queue when the default exchange routes
// by routing key) — and hands the context down. It emits nothing itself:
// emission happens in the governance layer under it. [NewObsPublisher] builds
// it.
type ObsPublisher struct {
	next InnerPublisher
}

// NewObsPublisher builds the identity layer over next.
func NewObsPublisher(next InnerPublisher) *ObsPublisher { return &ObsPublisher{next: next} }

// Release hands releaseRaw to the layer under it — this layer holds no
// resource.
func (o *ObsPublisher) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

func (o *ObsPublisher) Publish(ctx context.Context, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	dest := exchange
	if dest == "" {
		// The default exchange routes by queue name carried in the routing key.
		dest = key
	}
	return o.next.Publish(observability.WithOperation(ctx, operation(opPublish, dest)),
		ch, exchange, key, mandatory, immediate, pub)
}

// GuardedPublish publishes pub to exchange/routingKey on ch, routed through
// the client's chain when governance is enabled. When governance is disabled
// this behaves exactly like ch.PublishWithContext. On rejection (rate-limit or
// open circuit) the returned error is a resilience sentinel and the underlying
// publish is never invoked.
//
// It is a thin convenience over the client's embedded [InnerPublisher] head;
// the publish declares its identity, and the publish is driven inside the
// executor, so pub is a copy — a caller that needs the injected headers back
// on its own Publishing should publish directly instead.
func GuardedPublish(ctx context.Context, cl *Client, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	return cl.InnerPublisher.Publish(ctx, ch, exchange, key, mandatory, immediate, pub)
}
