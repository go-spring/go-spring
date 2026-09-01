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
// (observe.Observer-backed publish/consume spans) and the resilience guard
// (executor-backed GuardedPublish) that wrap the raw mqtt.Client's operations.
// paho.mqtt.golang ships no hook/plugin extension point, so instead of a
// transparent client wrapper the seam is opt-in helpers the caller wraps around
// Publish and inside the Subscribe callback.
package StarterMQTT

import (
	"context"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
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
// + access log) via the shared observe kit, riding the OTel globals starter-otel
// installs. Call them around Publish and inside the Subscribe callback.

// The observers are built lazily (sync.Once) so a blank import of this starter
// no longer pays the observe-kit construction at package init — only apps that
// actually call the span helpers build them.
//
// Because the span helpers are package-level (they take no client, unlike the
// nats Conn methods), the observers cannot be per-instance. Instead the first
// configured client seeds their ObserveConfig from its
// ${spring.mqtt.<name>.observability} entry (see seedObserveConfig in
// starter.go wiring); the helpers honor that configured level. Apps using the
// helpers without any starter-configured client get the observe-kit default
// (brief).
var (
	defaultObsOnce sync.Once
	obsCfgMu       sync.Mutex
	obsCfgSeeded   bool
	obsCfg         = observe.ObserveConfig{Level: observe.DefaultBrief}
	pubObs         *observe.Observer
	subObs         *observe.Observer
)

// seedObserveConfig records the observability config the lazy observers will be
// built with. Called once per process by the first client the starter wires, so
// the span helpers honor ${spring.mqtt.<name>.observability.level} rather than
// always defaulting to brief. Only the first client seeds: with multiple
// clients configured differently there is no single right answer for a
// package-level observer, so first-wins is the deterministic, documented rule.
// Has no effect if the observers were already built (helpers used before any
// client was wired).
func seedObserveConfig(cfg observe.ObserveConfig) {
	obsCfgMu.Lock()
	defer obsCfgMu.Unlock()
	if cfg.Level == "" {
		return // nothing configured; keep the kit default
	}
	if !obsCfgSeeded {
		obsCfg = cfg
		obsCfgSeeded = true
	}
}

func pubObserver() *observe.Observer {
	defaultObsOnce.Do(func() {
		obsCfgMu.Lock()
		cfg := obsCfg
		obsCfgMu.Unlock()
		pubObs = observe.NewProducer("mqtt", cfg)
		subObs = observe.NewConsumer("mqtt", cfg)
	})
	return pubObs
}

func subObserver() *observe.Observer {
	defaultObsOnce.Do(func() {
		obsCfgMu.Lock()
		cfg := obsCfg
		obsCfgMu.Unlock()
		pubObs = observe.NewProducer("mqtt", cfg)
		subObs = observe.NewConsumer("mqtt", cfg)
	})
	return subObs
}

// StartPublishSpan opens a producer observation for a publish to topic. Call
// right before client.Publish and End the returned span once the token resolves:
//
//	ctx, sp := StarterMQTT.StartPublishSpan(ctx, "sensors/temp")
//	tok := client.Publish("sensors/temp", qos, false, payload)
//	_ = tok.Wait()
//	StarterMQTT.EndSpan(sp, tok.Error())
func StartPublishSpan(ctx context.Context, topic string) (context.Context, *observe.Span) {
	return pubObserver().Start(ctx, "publish", topic)
}

// StartConsumeSpan opens a consumer observation for an inbound message. Call at
// the top of a subscription callback and End once handling finishes:
//
//	sub, _ := client.Subscribe("sensors/temp", qos, func(c mqtt.Client, m mqtt.Message) {
//	    ctx, sp := StarterMQTT.StartConsumeSpan(ctx, m)
//	    err := handle(ctx, m)
//	    StarterMQTT.EndSpan(sp, err)
//	})
func StartConsumeSpan(ctx context.Context, msg mqtt.Message) (context.Context, *observe.Span) {
	return subObserver().Start(ctx, "consume", msg.Topic())
}

// EndSpan records err (if any) on the span and ends it.
func EndSpan(span *observe.Span, err error) {
	span.End(err)
}

// resilienceExecs tracks the resilience executor attached to each client, so
// GuardedPublish can resolve it from the raw mqtt.Client bean and the destructor
// can Close it (releasing any background resources of a production driver).
// Only clients with resilience enabled appear here.
var resilienceExecs sync.Map // mqtt.Client -> resilience.Executor

// resilienceResources tracks the stable resource label per client so the guard
// can pass it to exec.Execute without re-deriving from Config.
var resilienceResources sync.Map // mqtt.Client -> string

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
func applyResilience(c Config, cl mqtt.Client, resource string) error {
	// Per-instance opt-out: without an executor attached, guard (and therefore
	// both GuardedPublish and the binder's Publish) degrades to bare calls.
	if !c.Governance {
		return nil
	}
	exec := fault.WrapExecutor(resilience.ExecutorFor(resource))
	exec = resilience.WrapExecutor(exec, "mqtt", c.Observability)
	resilienceExecs.Store(cl, exec)
	resilienceResources.Store(cl, resource)
	return nil
}

// closeResilience closes and forgets the executor behind cl, if any.
func closeResilience(cl mqtt.Client) {
	if v, ok := resilienceExecs.LoadAndDelete(cl); ok {
		_ = v.(resilience.Executor).Close()
	}
	resilienceResources.Delete(cl)
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, cl mqtt.Client, call func(context.Context) error) error {
	v, ok := resilienceExecs.Load(cl)
	if !ok {
		return call(ctx)
	}
	r, _ := resilienceResources.Load(cl)
	return v.(resilience.Executor).Execute(ctx, r.(string), call)
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
