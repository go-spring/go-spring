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
//	resilience — applyResilience + guard drive the backend-neutral executor
//	             through the opt-in GuardedPublish call site, since amqp091
//	             exposes no reject-capable middleware. The executor is also the
//	             single emitter: it reads the declared operation off the ctx and
//	             opens the span, records the durations (call-level and
//	             attempt-level) and writes the one access log.
package StarterRabbitMQ

import (
	"context"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/governance"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
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

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable serviceLabel it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/serviceLabel combination).
type clientGuard struct {
	exec         chain.Executor
	serviceLabel string
}

// clientGuards indexes the guard by the raw client bean, so GuardedPublish can resolve
// it from a bare *amqp.Connection and the destructor can Close it. Only clients with
// resilience enabled appear here.
var clientGuards sync.Map // *amqp.Connection -> *clientGuard

// applyResilience builds the connection's guarded executor from the governance
// center's resilience authority, wrapped with its fault authority.
func applyResilience(conn *amqp.Connection, serviceLabel string, center *governance.Center) error {
	exec := fault.WrapClientExecutor(center.Resilience().ClientExecutorFor("rabbitmq", serviceLabel), serviceLabel, center.Fault())
	clientGuards.Store(conn, &clientGuard{exec: exec, serviceLabel: serviceLabel})
	return nil
}

// closeResilience closes and forgets the executor behind conn, if any.
func closeResilience(conn *amqp.Connection) {
	if v, ok := clientGuards.LoadAndDelete(conn); ok {
		if err := v.(*clientGuard).exec.Close(); err != nil {
			log.Warnf(context.Background(), log.TagAppDef, "rabbitmq: resilience executor close failed: %v", err)
		}
	}
}

// guard routes call through the executor attached to conn, and otherwise runs it
// inline. When resilience is disabled for the connection this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, conn *amqp.Connection, call func(context.Context) error) error {
	v, ok := clientGuards.Load(conn)
	if !ok {
		return call(ctx)
	}
	g := v.(*clientGuard)
	return g.exec.Execute(ctx, call)
}

// GuardedPublish publishes pub to exchange/routingKey on ch, routed through the
// resilience executor attached to conn when governance is enabled.
// When governance is disabled this behaves exactly like ch.PublishWithContext.
// On rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying publish is never invoked.
//
// It declares the publish's identity (see [operation]) on the ctx before running
// the call, so the executor emits the span, the durations and the access log; it
// also injects the current W3C trace context into pub.Headers from the attempt
// ctx the executor hands inward, so the traceparent carries the executor's span
// and links the broker trace to this call. The publish is driven inside the
// executor, so pub is a copy — a caller that needs the injected headers back on
// its own Publishing should publish directly instead.
//
// The connection (not the channel) is passed to resolve the executor because a
// channel may outlive the connection bean in some patterns, while the executor
// is always scoped to the connection the starter created.
func GuardedPublish(ctx context.Context, conn *amqp.Connection, ch *amqp.Channel, exchange, key string, mandatory, immediate bool, pub amqp.Publishing) error {
	dest := exchange
	if dest == "" {
		// The default exchange routes by queue name carried in the routing key.
		dest = key
	}
	ctx = observability.WithOperation(ctx, operation(opPublish, dest))
	return guard(ctx, conn, func(attemptCtx context.Context) error {
		injectW3C(attemptCtx, &pub)
		return ch.PublishWithContext(attemptCtx, exchange, key, mandatory, immediate, pub)
	})
}
