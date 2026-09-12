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

// command.go is the "command seam" concept of this starter: the observe layer
// (module-local publish/consume observers, see observe.go) and the resilience
// guard (executor-backed GuardedPublish) that wrap the raw mqtt.Client's
// operations. paho.mqtt.golang ships no hook/plugin extension point, so instead
// of a transparent client wrapper the seam is opt-in helpers the caller wraps
// around Publish and inside the Subscribe callback.
package StarterMQTT

import (
	"context"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go.opentelemetry.io/otel/trace"
)

// MQTT observability is driven by these kit-backed helpers rather than a
// transparent client wrapper, for two reasons:
//
//  1. paho.mqtt.golang (v1) ships no OTel instrumentation, and Publish returns
//     an async Token whose error/timing is not known at the call site — a
//     transparent wrapper would have to wrap the Token too, which is fragile.
//
//  2. MQTT 3.1.1 (what paho v1 speaks) carries no message properties, so W3C
//     trace context cannot propagate across the broker: publish and consume
//     spans are independent traces. That is an inherent protocol limitation,
//     not an instrumentation gap.
//
// The helpers emit the full three-signal trio (span + duration/in-flight metric
// + access log) via the module-local observers in observe.go, riding the OTel
// globals starter-otel installs. Call them around Publish and inside the
// Subscribe callback.

// The observers are package-level (the span helpers take no client, unlike the
// nats Conn methods) and config-free. They are built at wiring time — the first
// client the starter wires constructs them (see newClient) — falling back to a
// lazy sync.Once for apps that call the helpers without any starter-configured
// client.
var (
	defaultObsOnce sync.Once
	pubObs         *observer
	subObs         *observer
)

// buildObservers constructs the span helpers' observers from the current OTel
// meter provider. Safe to call multiple times; only the first call wins.
func buildObservers() {
	defaultObsOnce.Do(func() {
		pubObs = newObserver(trace.SpanKindProducer)
		subObs = newObserver(trace.SpanKindConsumer)
	})
}

// StartPublishSpan opens a producer observation for a publish to topic. Call
// right before client.Publish and End the returned span once the token resolves:
//
//	ctx, sp := StarterMQTT.StartPublishSpan(ctx, "sensors/temp")
//	tok := client.Publish("sensors/temp", qos, false, payload)
//	_ = tok.Wait()
//	StarterMQTT.EndSpan(sp, tok.Error())
func StartPublishSpan(ctx context.Context, topic string) (context.Context, *span) {
	buildObservers()
	return pubObs.Start(ctx, "publish", topic)
}

// StartConsumeSpan opens a consumer observation for an inbound message. Call at
// the top of a subscription callback and End once handling finishes:
//
//	sub, _ := client.Subscribe("sensors/temp", qos, func(c mqtt.Client, m mqtt.Message) {
//	    ctx, sp := StarterMQTT.StartConsumeSpan(ctx, m)
//	    err := handle(ctx, m)
//	    StarterMQTT.EndSpan(sp, err)
//	})
func StartConsumeSpan(ctx context.Context, msg mqtt.Message) (context.Context, *span) {
	buildObservers()
	return subObs.Start(ctx, "consume", msg.Topic())
}

// EndSpan records err (if any) on the span and ends it.
func EndSpan(span *span, err error) {
	span.End(err)
}

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable resource label it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/resource combination).
type clientGuard struct {
	exec     resilience.Executor
	resource string
}

// clientGuards indexes the guard by the raw client bean, so GuardedPublish can resolve
// it from a bare mqtt.Client and the destructor can Close it. Only clients with
// resilience enabled appear here.
var clientGuards sync.Map // mqtt.Client -> *clientGuard

// applyResilience builds an executor and indexes it by cl. This is the mqtt seam
// of resilience. paho's Publish hands the message to the client's internal
// outbound queue and returns a Token; the caller then blocks on token.Wait().
// For QoS 0 Wait() returns once the packet is written; for QoS 1/2 it blocks
// until the PUBACK/PUBCOMP. Because paho manages its own queueing and reconnect,
// the executor here is intentionally minimal — rate limiting the publish rate
// and short-circuiting (circuit breaker) when the broker is unhealthy. It is
// driven through an opt-in call-site guard (GuardedPublish).
//
// The executor is resolved through the neutral [resilience.ExecutorFor] seam,
// which starter-govern backs with the governance center — so this function has
// zero coupling to cloud/governance. When governance is off, ExecutorFor yields a
// transparent no-op executor; fault wraps it when enabled.
func applyResilience(cl mqtt.Client, resource string) error {
	exec := fault.WrapExecutor(resilience.ExecutorFor("mqtt", resource))
	clientGuards.Store(cl, &clientGuard{exec: exec, resource: resource})
	return nil
}

// closeResilience closes and forgets the executor behind cl, if any.
func closeResilience(cl mqtt.Client) {
	if v, ok := clientGuards.LoadAndDelete(cl); ok {
		_ = v.(*clientGuard).exec.Close()
	}
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, cl mqtt.Client, call func(context.Context) error) error {
	v, ok := clientGuards.Load(cl)
	if !ok {
		return call(ctx)
	}
	g := v.(*clientGuard)
	return g.exec.Execute(ctx, g.resource, call)
}

// GuardedPublish publishes payload to topic at qos, routed through the
// resilience executor attached to cl when governance is enabled.
// When governance is disabled this behaves exactly like a plain Client.Publish
// followed by token.Wait(). On rejection (rate-limit or open circuit) the
// returned error is a resilience sentinel and the underlying publish is never
// invoked.
//
// retained controls broker-side retention, matching the paho Publish signature.
// The function blocks until paho acknowledges the outbound handoff (immediately
// at QoS 0, after a PUBACK/PUBCOMP at QoS 1/2).
func GuardedPublish(ctx context.Context, cl mqtt.Client, topic string, qos byte, retained bool, payload interface{}) error {
	return guard(ctx, cl, func(context.Context) error {
		token := cl.Publish(topic, qos, retained, payload)
		token.Wait()
		return token.Error()
	})
}
