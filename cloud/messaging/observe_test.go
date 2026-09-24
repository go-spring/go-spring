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

package messaging_test

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/messaging"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// fakeDriver is an in-memory Driver: the publisher records what it was given
// (and may fail on script), the subscriber parks its handler so the test
// delivers messages by hand.
type fakeDriver struct {
	publishErr error
	published  []*messaging.Message
	handler    messaging.Handler
}

func (d *fakeDriver) NewPublisher(context.Context, string) (messaging.Publisher, error) {
	return d, nil
}

func (d *fakeDriver) NewSubscriber(context.Context, string, string) (messaging.Subscriber, error) {
	return d, nil
}

func (d *fakeDriver) Publish(_ context.Context, msg *messaging.Message) error {
	d.published = append(d.published, msg)
	return d.publishErr
}

func (d *fakeDriver) Subscribe(_ context.Context, handler messaging.Handler) error {
	d.handler = handler
	return nil
}

func (d *fakeDriver) Close() error { return nil }

// withProviders installs a manual meter reader and a recording tracer
// provider, plus the W3C trace-context propagator (the global default is
// none). Since Observe builds its instruments at construction time, call
// withProviders before Observe.
func withProviders(t *testing.T) (*sdkmetric.ManualReader, *tracetest.SpanRecorder) {
	t.Helper()
	rdr := sdkmetric.NewManualReader()
	prevMeter := otel.GetMeterProvider()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr)))

	rec := tracetest.NewSpanRecorder()
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))

	prevProp := otel.GetTextMapPropagator()
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = rdr.Shutdown(context.Background())
		otel.SetMeterProvider(prevMeter)
		otel.SetTracerProvider(prevTracer)
		otel.SetTextMapPropagator(prevProp)
	})
	return rdr, rec
}

// inFlight collects the current value of messaging.operation.active.
func inFlight(t *testing.T, rdr *sdkmetric.ManualReader) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "messaging.operation.active" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				return dp.Value
			}
		}
	}
	return 0
}

// totals collects messaging.operation.total into a "operation.status" -> count map.
func totals(t *testing.T, rdr *sdkmetric.ManualReader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.That(t, rdr.Collect(context.Background(), &rm)).Nil()
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "messaging.operation.total" {
				continue
			}
			for _, dp := range m.Data.(metricdata.Sum[int64]).DataPoints {
				op, status := "", ""
				for _, kv := range dp.Attributes.ToSlice() {
					if kv.Key == "messaging.operation" {
						op = kv.Value.AsString()
					}
					if kv.Key == "status" {
						status = kv.Value.AsString()
					}
				}
				out[op+"."+status] = dp.Value
			}
		}
	}
	return out
}

func TestObservePropagatesTraceContext(t *testing.T) {
	_, rec := withProviders(t)
	fake := &fakeDriver{}
	d := messaging.Observe(fake, "fake")

	pub, err := d.NewPublisher(context.Background(), "orders")
	assert.That(t, err).Nil()
	sub, err := d.NewSubscriber(context.Background(), "orders", "")
	assert.That(t, err).Nil()
	var handled trace.Span
	assert.That(t, sub.Subscribe(context.Background(), func(ctx context.Context, _ *messaging.Message) error {
		handled = trace.SpanFromContext(ctx)
		return nil
	})).Nil()

	ctx, root := otel.Tracer("test").Start(context.Background(), "root")
	assert.That(t, pub.Publish(ctx, &messaging.Message{Payload: []byte("x")})).Nil()
	root.End()

	// The envelope carried the W3C trace context to the consumer side...
	assert.That(t, fake.published[0].Header("traceparent") != "").True()

	// ...so the consumer span continues the producer's trace: publish and
	// consume share the root's trace ID, and the handler ran inside the span.
	fake.handler(context.Background(), fake.published[0])
	spans := rec.Ended()
	var consume sdktrace.ReadOnlySpan
	for _, s := range spans {
		if s.Name() == "consume" {
			consume = s
		}
	}
	assert.That(t, consume != nil).True()
	assert.That(t, consume.SpanContext().TraceID() == root.SpanContext().TraceID()).True()
	assert.That(t, handled.SpanContext().SpanID() == consume.SpanContext().SpanID()).True()
}

func TestObserveStatuses(t *testing.T) {
	rdr, _ := withProviders(t)
	fake := &fakeDriver{publishErr: errors.New("boom")}
	d := messaging.Observe(fake, "fake")

	pub, err := d.NewPublisher(context.Background(), "orders")
	assert.That(t, err).Nil()
	sub, err := d.NewSubscriber(context.Background(), "orders", "")
	assert.That(t, err).Nil()

	// A failed publish and a successful one; the error passes through unchanged.
	assert.That(t, pub.Publish(context.Background(), &messaging.Message{}) != nil).True()
	fake.publishErr = nil
	assert.That(t, pub.Publish(context.Background(), &messaging.Message{})).Nil()

	// A failed consume and a successful one; the handler's error passes
	// through unchanged too — error policy stays the driver's.
	sentinel := errors.New("handler boom")
	assert.That(t, sub.Subscribe(context.Background(), func(_ context.Context, _ *messaging.Message) error {
		return sentinel
	})).Nil()
	err = fake.handler(context.Background(), &messaging.Message{})
	assert.That(t, errors.Is(err, sentinel)).True()

	assert.That(t, sub.Subscribe(context.Background(), func(context.Context, *messaging.Message) error {
		return nil
	})).Nil()
	assert.That(t, fake.handler(context.Background(), &messaging.Message{})).Nil()

	got := totals(t, rdr)
	assert.That(t, got["publish.error"]).Equal(int64(1))
	assert.That(t, got["publish.ok"]).Equal(int64(1))
	assert.That(t, got["consume.error"]).Equal(int64(1))
	assert.That(t, got["consume.ok"]).Equal(int64(1))
}

func TestObservePanickingHandlerDoesNotLeak(t *testing.T) {
	rdr, rec := withProviders(t)
	fake := &fakeDriver{}
	d := messaging.Observe(fake, "fake")

	sub, err := d.NewSubscriber(context.Background(), "orders", "")
	assert.That(t, err).Nil()
	assert.That(t, sub.Subscribe(context.Background(), func(context.Context, *messaging.Message) error {
		panic("boom")
	})).Nil()

	func() {
		defer func() { _ = recover() }() // the panic-guard's job, not the decorator's
		_ = fake.handler(context.Background(), &messaging.Message{})
	}()

	// The panic still propagated (recovered here), yet the in-flight gauge is
	// back to zero and the consume span was closed — no leaked instrumentation.
	assert.That(t, inFlight(t, rdr)).Equal(int64(0))
	ended := false
	for _, s := range rec.Ended() {
		if s.Name() == "consume" {
			ended = true
		}
	}
	assert.That(t, ended).True()
}
