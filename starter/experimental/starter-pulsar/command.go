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

// command.go is the "command/operation seam" concept of this starter: the
// observe layer (native Prometheus metrics + OTel tracing helpers + the driver's
// per-message observers) and the resilience guard (GuardedSend plus the
// per-client executor index). pulsar-client-go exposes no reject-capable
// middleware and producers are caller-created, so the guard is an opt-in
// call-site wrapper on the synchronous Producer.Send path.
package StarterPulsar

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// -----------------------------------------------------------------------------
// Metrics (native Prometheus)
// -----------------------------------------------------------------------------

// newMetricsServer builds a dedicated Prometheus registry for one pulsar client
// and starts a standalone HTTP server rendering it on cfg.Path. The registry is
// returned so the caller can wire it into ClientOptions.MetricsRegisterer; the
// server is returned so it can be shut down when the client is destroyed.
//
// A per-instance registry (rather than the process-wide DefaultRegisterer) keeps
// multiple pulsar clients from colliding on identical pulsar_client_* metric
// names, and keeps these raw Prometheus metrics cleanly separate from the OTel
// SDK registry that starter-otel manages.
func newMetricsServer(cfg MetricsConfig) (prometheus.Registerer, *http.Server) {
	reg := prometheus.NewRegistry()
	mux := http.NewServeMux()
	mux.Handle(cfg.Path, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Warnf(context.Background(), log.TagAppDef, "pulsar: metrics server exited unexpectedly: %v", err)
		}
	}()
	return reg, srv
}

// -----------------------------------------------------------------------------
// Tracing (native OTel helpers)
// -----------------------------------------------------------------------------

// pulsar-client-go has no OTel contrib and no span injection point of its own,
// so message-level tracing is done here with small call-site helpers built on
// the OTel API. They ride the global TracerProvider and propagator that
// starter-otel installs; without it they are no-ops and touch no message bytes.
//
// pulsar carries the W3C trace context in the message Properties map, which is
// delivered verbatim to consumers, so producer and consumer spans link across
// services the same way the HTTP/Kafka paths do.

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-pulsar"

// StartProducerSpan starts a producer span for msg and injects the current W3C
// trace context into msg.Properties. Call it right before Producer.Send and end
// the returned span once the send completes:
//
//	ctx, span := StarterPulsar.StartProducerSpan(ctx, msg)
//	_, err := producer.Send(ctx, msg)
//	StarterPulsar.EndSpan(span, err)
func StartProducerSpan(ctx context.Context, msg *pulsar.ProducerMessage) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "pulsar.produce",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "pulsar"),
			attribute.String("messaging.operation", "publish"),
		),
	)
	if msg.Properties == nil {
		msg.Properties = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Properties))
	return ctx, span
}

// StartConsumerSpan extracts the upstream trace context carried in msg's
// properties and starts a consumer span as its child. Call it when a message is
// received and end the returned span once processing finishes:
//
//	ctx, span := StarterPulsar.StartConsumerSpan(ctx, msg)
//	err := handle(ctx, msg)
//	StarterPulsar.EndSpan(span, err)
func StartConsumerSpan(ctx context.Context, msg pulsar.Message) (context.Context, trace.Span) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Properties()))
	tracer := otel.GetTracerProvider().Tracer(tracerName)
	ctx, span := tracer.Start(ctx, "pulsar.consume "+msg.Topic(),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "pulsar"),
			attribute.String("messaging.destination.name", msg.Topic()),
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

// --- driver auto path (observed) ----------------------------------------------
//
// The messaging.Driver drives produce/consume through the module-local observer
// in observe.go (span+metric+access log). Pulsar's own Prometheus metrics (see
// newMetricsServer above) are separate and stay — they are library-native
// connection/producer stats, while the observer covers per-message traffic. The
// manual helpers above remain for apps that want explicit span control.

// The observers are built lazily (sync.Once) so a blank import of this starter
// pays no OTel instrument creation at package init — only apps that actually
// publish or consume through the driver path build them.
var (
	defaultObsOnce sync.Once
	defaultPubObs  *observer
	defaultSubObs  *observer
)

func pubObserver() *observer {
	defaultObsOnce.Do(func() {
		defaultPubObs = newObserver(trace.SpanKindProducer)
		defaultSubObs = newObserver(trace.SpanKindConsumer)
	})
	return defaultPubObs
}

func subObserver() *observer {
	defaultObsOnce.Do(func() {
		defaultPubObs = newObserver(trace.SpanKindProducer)
		defaultSubObs = newObserver(trace.SpanKindConsumer)
	})
	return defaultSubObs
}

// startProduce opens a producer observation and injects W3C trace context into
// msg.Properties. topic is the producer's destination (the driver passes it).
func startProduce(ctx context.Context, topic string, msg *pulsar.ProducerMessage) (context.Context, obsSpan) {
	ctx, sp := pubObserver().Start(ctx, "publish", topic)
	if msg.Properties == nil {
		msg.Properties = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Properties))
	return ctx, sp
}

// startConsume extracts the upstream trace from the message properties and opens
// a consumer observation. For the driver's consume loop.
func startConsume(ctx context.Context, msg pulsar.Message) (context.Context, obsSpan) {
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(msg.Properties()))
	return subObserver().Start(ctx, "consume", msg.Topic())
}

// -----------------------------------------------------------------------------
// Resilience guard
// -----------------------------------------------------------------------------

// clientGuard is the per-client resilience attachment: the executor chain and
// the stable resource label it executes under, colocated so a guard lookup
// reads the pair atomically (no torn exec/resource combination).
type clientGuard struct {
	exec     resilience.Executor
	resource string
}

// clientGuards indexes the guard by the raw client bean, so GuardedSend can resolve
// it from a bare pulsar.Client and the destructor can Close it. Only clients with
// resilience enabled appear here.
var clientGuards sync.Map // pulsar.Client -> *clientGuard

// applyResilience builds an executor and indexes it by cl. This is the pulsar
// seam of resilience: pulsar-client-go exposes no reject-capable middleware and
// producers are caller-created, so the executor is driven through an opt-in
// call-site guard (GuardedSend) on the synchronous Producer.Send path.
//
// Both the executor and the fault injector are resolved through neutral seams
// ([resilience.ExecutorFor] / [fault.InjectorFor]) that starter-govern backs with
// the governance center — so this function has zero coupling to cloud/governance.
// When governance is off, ExecutorFor yields a transparent no-op executor; fault
// wraps it when an injector is registered (nil-safe otherwise).
func applyResilience(c Config, cl pulsar.Client, resource string) error {
	// Per-instance opt-out: without an executor attached, guard (and therefore
	// both GuardedSend and the driver's Publish) degrades to bare calls.
	if !c.Governance {
		return nil
	}
	exec := fault.WrapExecutor(resilience.ExecutorFor(resource))
	exec = resilience.WrapExecutor(exec, "pulsar")
	clientGuards.Store(cl, &clientGuard{exec: exec, resource: resource})
	return nil
}

// closeResilience closes and forgets the executor behind cl, if any.
func closeResilience(cl pulsar.Client) {
	if v, ok := clientGuards.LoadAndDelete(cl); ok {
		if err := v.(*clientGuard).exec.Close(); err != nil {
			log.Warnf(context.Background(), log.TagAppDef, "pulsar: resilience executor close failed: %v", err)
		}
	}
}

// guard routes call through the executor attached to cl, and otherwise runs it
// inline. When resilience is disabled for the client this is a no-op
// pass-through, so enabling protection is a zero-code opt-in on the caller side.
func guard(ctx context.Context, cl pulsar.Client, call func(context.Context) error) error {
	v, ok := clientGuards.Load(cl)
	if !ok {
		return call(ctx)
	}
	g := v.(*clientGuard)
	return g.exec.Execute(ctx, g.resource, call)
}

// GuardedSend sends msg synchronously on producer, routed through the resilience
// executor attached to cl when governance is enabled. When
// governance is disabled this behaves exactly like producer.Send. On rejection
// (rate-limit or open circuit) the returned error is a resilience sentinel and
// the underlying send is never invoked.
//
// The client (not the producer) is passed to resolve the executor because
// producers are caller-created and may be recreated over a client's lifetime,
// while the executor is always scoped to the client the starter created. The
// synchronous Producer.Send blocks until the broker acknowledges, which is the
// path worth protecting; the asynchronous SendAsync is intentionally untouched.
func GuardedSend(ctx context.Context, cl pulsar.Client, producer pulsar.Producer, msg *pulsar.ProducerMessage) (pulsar.MessageID, error) {
	var id pulsar.MessageID
	err := guard(ctx, cl, func(ctx context.Context) error {
		var serr error
		id, serr = producer.Send(ctx, msg)
		return serr
	})
	if err != nil {
		return nil, err
	}
	return id, nil
}
