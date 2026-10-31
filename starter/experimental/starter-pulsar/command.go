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
// operation declaration + protection that wraps publishes and consumes, the
// native Prometheus metrics endpoint, and the app-facing manual tracing helpers.
// Three concerns live here:
//
//	declare    — a publish or consume declares the operation's identity (see
//	             [operation]) on the ctx and moves the W3C trace context across
//	             the broker's message properties. The span, the metrics and the
//	             access log are NOT emitted here: declaring the identity is this
//	             layer's whole job now, and the resilience executor emits from
//	             the one point on the chain that sees a whole call, retries
//	             included.
//	resilience — AttachGovernance + guard drive the backend-neutral executor
//	             through the opt-in GuardedSend call site, since pulsar-client-go
//	             exposes no reject-capable middleware and producers are
//	             caller-created. The executor is attached at construction
//	             (AttachGovernance) and is also the single emitter: it reads the
//	             declared operation off the ctx and opens the span, records the
//	             durations (call-level and attempt-level) and writes the one
//	             access log.
//	metrics    — the native pulsar_client_* Prometheus registry and its
//	             per-instance /metrics server. These are library-native
//	             connection/producer/consumer stats, not per-call signals, so
//	             they stay here rather than moving to the resilience layer.

package StarterPulsar

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go-spring.org/cloud/governance/capability/label"
	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	observability "go-spring.org/cloud/governance/dataplane/outbound/observe"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
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
		if err := srv.ListenAndServe(); err != nil && !errutil.IsServerClosed(err) {
			log.Warn(context.Background(), log.TagAppDef, "pulsar: metrics server exited unexpectedly", log.Err(err))
		}
	}()
	return reg, srv
}

// -----------------------------------------------------------------------------
// Tracing (native OTel helpers)
// -----------------------------------------------------------------------------

// pulsar-client-go has no OTel contrib and no span injection point of its own,
// so message-level tracing is done here with small call-site helpers built on
// the OTel API. This is the app's manual path for a raw send it drives itself;
// a publish or consume that goes through the driver-agnostic seam (GuardedSend,
// or the driver's own publish/consume) DECLARES its operation instead and the
// resilience layer emits the span — see [operation] and the resilience guard.
// The W3C trace context still rides the message Properties map, delivered
// verbatim to consumers, so producer and consumer spans link across services
// the same way the HTTP/Kafka paths do.
//
// The helpers ride the global TracerProvider and propagator that starter-otel
// installs; without it they are no-ops and touch no message bytes.

// componentName is the instrumentation componentName name every meter and tracer in this package reports under.
const componentName = "go-spring.org/starter-pulsar"

// injectTraceContext inserts the current W3C trace context into msg.Properties,
// so a subscriber can continue the trace across the broker. With no valid span
// on ctx (the ungoverned path) it writes nothing meaningful.
func injectTraceContext(ctx context.Context, msg *pulsar.ProducerMessage) {
	if msg.Properties == nil {
		msg.Properties = make(map[string]string)
	}
	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(msg.Properties))
}

// extractTraceContext pulls the upstream trace context out of props. It returns
// the input ctx unchanged when nothing was propagated.
func extractTraceContext(ctx context.Context, props map[string]string) context.Context {
	if len(props) == 0 {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier(props))
}

// StartProducerSpan starts a producer span for msg and injects the current W3C
// trace context into msg.Properties. Call it right before Producer.Send and end
// the returned span once the send completes:
//
//	ctx, span := StarterPulsar.StartProducerSpan(ctx, msg)
//	_, err := producer.Send(ctx, msg)
//	StarterPulsar.EndSpan(span, err)
//
// This is the app's own manual span for a raw send it drives directly; a call
// routed through [GuardedSend] is spanned by the resilience layer instead (from
// the operation GuardedSend declares), so do not wrap both around the same send.
func StartProducerSpan(ctx context.Context, msg *pulsar.ProducerMessage) (context.Context, trace.Span) {
	tracer := otel.GetTracerProvider().Tracer(componentName)
	ctx, span := tracer.Start(ctx, "pulsar.produce",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", pulsarSystem),
			attribute.String("messaging.operation", "send"),
		),
	)
	injectTraceContext(ctx, msg)
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
	ctx = extractTraceContext(ctx, msg.Properties())
	tracer := otel.GetTracerProvider().Tracer(componentName)
	ctx, span := tracer.Start(ctx, "pulsar.consume "+msg.Topic(),
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", pulsarSystem),
			attribute.String("messaging.destination.name", msg.Topic()),
			attribute.String("messaging.operation", "process"),
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

// -----------------------------------------------------------------------------
// Resilience guard
// -----------------------------------------------------------------------------

// clientObserver is the per-client emission attachment: the observer and the
// stable serviceLabel it reports under, colocated so a lookup reads the pair
// atomically (no torn observer/serviceLabel combination).
type clientObserver struct {
	obs          *observability.OperationObserver
	serviceLabel string
}

// clientGuards indexes the observer by the raw client bean, so GuardedSend can
// resolve it from a bare pulsar.Client.
var clientGuards sync.Map // pulsar.Client -> *clientObserver

// AttachObserver fixes the client's service label and attaches its observer,
// called by the Driver while it builds the client (see [Driver.CreateClient]) so
// the client is complete when it is returned — there is no later patch step.
// This is the pulsar seam of observation: pulsar-client-go exposes no
// per-message middleware and producers are caller-created, so emission is driven
// through the opt-in call-site seam (GuardedSend) on the synchronous
// Producer.Send path. A pulsar client has no seam for the resilience executor,
// so there is no governance: the observer emits the span, metrics and access
// log, and nothing throttles or retries.
func AttachObserver(cl pulsar.Client, url string) {
	label := label.ServiceLabel(pulsarSystem, url)
	clientGuards.Store(cl, &clientObserver{
		obs:          observability.NewOperationObserver(pulsarSystem, label),
		serviceLabel: label,
	})
}

// forgetObserver drops the observer behind cl, if any.
func forgetObserver(cl pulsar.Client) {
	clientGuards.Delete(cl)
}

// observe runs call under the observer attached to cl; a client with no
// attachment (a driver that skipped [AttachObserver]) has the call run inline.
func observe(ctx context.Context, cl pulsar.Client, op chain.Operation, call func(context.Context) error) error {
	v, ok := clientGuards.Load(cl)
	if !ok {
		return call(ctx)
	}
	return v.(*clientObserver).obs.Observe(ctx, op, nil, call)
}

// GuardedSend sends msg synchronously on producer, routed through the resilience
// executor attached to cl (see [AttachGovernance]): the governed executor. On
// rejection (rate-limit
// or open circuit) the returned error is a resilience sentinel and the underlying
// send is never invoked.
//
// The publish declares its operation (topic as Detail, direction and system as
// Attrs) before the send, so the resilience executor — the single emitter —
// opens the span, records the durations and writes the access log from the
// declaration. The W3C trace context is injected from the attempt ctx the
// executor hands inward, so the traceparent carries the executor's span and
// links the broker trace to this call; with no executor guard runs the send
// inline and the injection carries whatever span the caller's ctx holds.
//
// The client (not the producer) is passed to resolve the executor because
// producers are caller-created and may be recreated over a client's lifetime,
// while the executor is always scoped to the client the starter created. The
// synchronous Producer.Send blocks until the broker acknowledges, which is the
// path worth protecting; the asynchronous SendAsync is intentionally untouched.
func GuardedSend(ctx context.Context, prop traffic.Propagator, cl pulsar.Client, producer pulsar.Producer, msg *pulsar.ProducerMessage) (pulsar.MessageID, error) {
	var id pulsar.MessageID
	err := observe(ctx, cl, operation(ctx, prop, opSend, producer.Topic()), func(attemptCtx context.Context) error {
		injectTraceContext(attemptCtx, msg)
		var serr error
		id, serr = producer.Send(attemptCtx, msg)
		return serr
	})
	if err != nil {
		return nil, err
	}
	return id, nil
}
