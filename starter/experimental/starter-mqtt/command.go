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
//	declare    — GuardedPublish / GuardedConsume declare the operation's
//	             identity (see [operation]) on the ctx before running under the
//	             executor.
//	resilience — AttachGovernance installs the executor the driver obtained from its
//	             governance bundle, and guard drives it through the opt-in call
//	             sites, since paho.mqtt.golang exposes no reject-capable
//	             middleware. The executor is also the single emitter: it reads
//	             the declared operation off the ctx and opens the span, records
//	             the durations (call-level and attempt-level) and writes the one
//	             access log.
package StarterMQTT

import (
	"context"
	"go-spring.org/cloud/chain"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// MQTT protection is driven by these opt-in call-site helpers rather than a
// transparent client wrapper, for two reasons:
//
//  1. paho.mqtt.golang (v1) ships no reject-capable middleware, and Publish
//     returns an async Token whose error/timing is not known at the call site —
//     a transparent wrapper would have to wrap the Token too, which is fragile.
//
//  2. MQTT 3.1.1 (what paho v1 speaks) carries no message properties, so W3C
//     trace context cannot propagate across the broker; a publish and a consume
//     are independent traces. That is an inherent protocol limitation, not an
//     instrumentation gap.
//
// The span, the metrics and the access log are not emitted here: declaring the
// identity is this layer's whole job now, and the resilience executor — the one
// point on the chain that sees a whole call — emits from that declaration.

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable serviceLabel it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/serviceLabel combination).
type clientGuard struct {
	exec         chain.Executor
	serviceLabel string
}

// clientGuards indexes the guard by the raw client, so GuardedPublish can resolve
// it from a bare mqtt.Client and the destructor can Close it. Every client a
// [Driver] builds appears here; a bare mqtt.Client that never went through a
// driver is absent and runs inline, unobserved.
var clientGuards sync.Map // mqtt.Client -> *clientGuard

// AttachGovernance installs the executor the client runs under, indexed by cl so
// [GuardedPublish] / [GuardedConsume] can resolve it from a bare mqtt.Client and
// [closeResilience] can Close it.
//
// The [Driver] calls it while it builds the client (see [Driver.CreateClient]),
// so the client is complete when returned and nothing patches it afterwards.
// paho's mqtt.Client is an interface the starter cannot add fields to, so the
// executor is held beside the client here rather than on it. The executor is the
// driver's params.ExecutorFor product — the governed one when the container is
// present, the observed-only one otherwise, never absent.
//
// paho's Publish hands the message to the client's internal outbound queue and
// returns a Token; the caller then blocks on token.Wait(). Because paho manages
// its own queueing and reconnect, the executor is intentionally minimal — rate
// limiting the publish rate and short-circuiting (circuit breaker) when the
// broker is unhealthy — and is driven through an opt-in call-site guard
// ([GuardedPublish]).
func AttachGovernance(cl mqtt.Client, broker string, params cloud.ClientParams) {
	label := resilience.ServiceLabel("mqtt", broker)
	clientGuards.Store(cl, &clientGuard{exec: params.ExecutorFor("mqtt", label), serviceLabel: label})
}

// closeResilience closes and forgets the executor behind cl, if any.
func closeResilience(cl mqtt.Client) {
	if v, ok := clientGuards.LoadAndDelete(cl); ok {
		_ = v.(*clientGuard).exec.Close()
	}
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. A client built through a [Driver] always carries one (the governed or
// the observed-only executor), so enabling protection is a zero-code opt-in on
// the caller side; a bare client that never went through a driver has none and
// the call runs inline.
func guard(ctx context.Context, cl mqtt.Client, call func(context.Context) error) error {
	v, ok := clientGuards.Load(cl)
	if !ok {
		return call(ctx)
	}
	g := v.(*clientGuard)
	return g.exec.Execute(ctx, call)
}

// GuardedPublish publishes payload to topic at qos, routed through the
// resilience executor attached to cl when governance is enabled. The publish is
// declared (see [operation]) so the executor emits its span, metrics and access
// log; with no executor (governance off) guard runs the publish inline, behaving
// exactly like a plain Client.Publish followed by token.Wait().
// On rejection (rate-limit or open circuit) the returned error is a resilience
// sentinel and the underlying publish is never invoked.
//
// retained controls broker-side retention, matching the paho Publish signature.
// The function blocks until paho acknowledges the outbound handoff (immediately
// at QoS 0, after a PUBACK/PUBCOMP at QoS 1/2).
func GuardedPublish(ctx context.Context, cl mqtt.Client, topic string, qos byte, retained bool, payload interface{}) error {
	ctx = observability.WithOperation(ctx, operation(opPublish, topic))
	return guard(ctx, cl, func(context.Context) error {
		token := cl.Publish(topic, qos, retained, payload)
		token.Wait()
		return token.Error()
	})
}

// GuardedConsume runs handler for one delivered message, declaring the consume
// operation (topic from msg.Topic()) and routing the handler through the
// resilience executor attached to cl. It is the consume-side counterpart of
// GuardedPublish, for callers that subscribe on the raw mqtt.Client and want the
// delivery observed and guarded: call it at the top of the paho subscription
// callback, passing the message paho handed in.
//
//	sub, _ := client.Subscribe("sensors/temp", qos, func(_ mqtt.Client, m mqtt.Message) {
//	    _ = StarterMQTT.GuardedConsume(ctx, client, m, func(ctx context.Context) error {
//	        return handle(ctx, m)
//	    })
//	})
//
// With no executor attached (governance off) the handler runs inline; the
// declared operation still reaches whatever emitter is on the chain. The paho
// callback is fire-and-forget, so the returned error is the caller's to log —
// there is no ack/nack for MQTT 3.1.1.
func GuardedConsume(ctx context.Context, cl mqtt.Client, msg mqtt.Message, handler func(context.Context) error) error {
	ctx = observability.WithOperation(ctx, operation(opConsume, msg.Topic()))
	return guard(ctx, cl, handler)
}
