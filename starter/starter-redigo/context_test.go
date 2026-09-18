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

package StarterRedigo

import (
	"context"
	"testing"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ctxAttrsProcessor mirrors ContextAttributesProcessor (starter-otel/trace/context.go)
// instead of importing it: this module deliberately has no starter-otel
// dependency, and adding one would drag the SDK, the exporters and the
// container into a redis client. What this test locks is the half that lives
// here — a span started by this package on the caller's behalf is started from
// the caller's ctx, so the attributes carried on it arrive without the caller
// ever holding a span object. The processor's own wiring is covered by
// starter-otel's tests.
type ctxAttrsProcessor struct{}

func (ctxAttrsProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if attrs := observability.ContextAttributes(parent); len(attrs) > 0 {
		s.SetAttributes(attrs...)
	}
}
func (ctxAttrsProcessor) OnEnd(sdktrace.ReadOnlySpan)      {}
func (ctxAttrsProcessor) Shutdown(context.Context) error   { return nil }
func (ctxAttrsProcessor) ForceFlush(context.Context) error { return nil }

// ctxStubConn implements the ctx-aware path, which is the one under test: the
// wrapper falls back to the context-less Do when the inner conn lacks
// DoContext, and that fallback roots the span in context.Background().
type ctxStubConn struct {
	redis.Conn
	reply      interface{}
	doContexts int
}

func (s *ctxStubConn) Do(string, ...interface{}) (interface{}, error) { return s.reply, nil }
func (s *ctxStubConn) DoContext(context.Context, string, ...interface{}) (interface{}, error) {
	s.doContexts++
	return s.reply, nil
}
func (s *ctxStubConn) Close() error                      { return nil }
func (s *ctxStubConn) Err() error                        { return nil }
func (s *ctxStubConn) Send(string, ...interface{}) error { return nil }
func (s *ctxStubConn) Flush() error                      { return nil }
func (s *ctxStubConn) Receive() (interface{}, error)     { return nil, nil }

// attrsOf flattens a span's attributes. Value.Emit, not AsString: AsString
// renders any non-STRING value as the empty string, which would make an
// assertion on a bool attribute pass vacuously.
func attrsOf(s sdktrace.ReadOnlySpan) map[string]string {
	m := make(map[string]string, len(s.Attributes()))
	for _, a := range s.Attributes() {
		m[string(a.Key)] = a.Value.Emit()
	}
	return m
}

// TestDoContextSpanCarriesContextAttributes locks the "reachable without
// holding the span" shape documented in cloud/observability/README.md: the
// command's span is created inside the interceptor chain, so a user layer -
// which is outermost - can only annotate the ctx it forwards. Both the
// caller's annotation and the layer's must end up on that span.
func TestDoContextSpanCarriesContextAttributes(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(ctxAttrsProcessor{}),
		sdktrace.WithSpanProcessor(sr),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	}()

	duration, active := newInstruments()
	inner := &ctxStubConn{reply: "v"}

	// The user's interceptor never sees a span: it annotates the ctx and
	// forwards, exactly as the README's recipe says.
	user := CommandInterceptor(func(next CommandHandler) CommandHandler {
		return func(ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
			ctx = observability.WithContextAttributes(ctx, attribute.String("tenant", "acme"))
			return next(ctx, cmd, args)
		}
	})
	c := NewConn(inner, user, observeInterceptor(duration, active))

	ctx := observability.WithContextAttributes(context.Background(),
		attribute.String("deployment", "canary"))
	if _, err := c.DoContext(ctx, "GET", "k"); err != nil {
		t.Fatalf("DoContext: %v", err)
	}

	assert.That(t, inner.doContexts).Equal(1)
	spans := sr.Ended()
	assert.That(t, len(spans)).Equal(1)
	got := attrsOf(spans[0])
	assert.That(t, got["tenant"]).Equal("acme")
	assert.That(t, got["deployment"]).Equal("canary")
	// The family's own attributes are still set on the same span.
	assert.That(t, got["db.system"]).Equal("redis")
}
