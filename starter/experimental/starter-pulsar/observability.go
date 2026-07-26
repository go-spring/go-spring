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

package StarterPulsar

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
			_ = err
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
