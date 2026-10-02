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

// Command example-otel shows where an attribute can reach the cache operation's
// span, and asserts each one lands.
//
// A span is started before the work and settled after it, so there are two
// moments to write into it. Inside that window only code holding the
// span-carrying context can write, and a layer of your own below [cache.New]
// holds it. Before the window there is no span to hold, so the write goes
// through an OpenTelemetry hook instead; this demo runs two of those, each
// under its own provider:
//
//	inside  a layer below cache.New                        the span handle
//	before  observability.WithSpanAttributes + reader   the framework's path
//	before  baggage + reader                               OTel's own carrier
//
// The hook is OpenTelemetry's in both; go-spring contributes the carrier the
// first rides on, and nothing to the second. Two further routes reach the same
// place and are left out: a Sampler, which is a single object serving every
// span in the process, and a wrapped TracerProvider, which replaces the entry
// point. Neither is for process-level dimensions: env, cluster and version are
// the same for every span and belong on the OTel resource, not on a per-span
// hook.
//
// The reader is a few lines, and starter-otel registers one already, so an
// application that lets it build the TracerProvider writes none of this.
//
// It self-asserts every step and exits non-zero on mismatch, so it doubles as
// the package's smoke test. No external service: every span sink is a recorder
// held by this process.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type user struct{ Name string }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("cache example-otel ok")
}

func run() error {
	if err := checkLayer(); err != nil {
		return err
	}
	if err := checkCarrier(); err != nil {
		return err
	}
	if err := checkBaggage(); err != nil {
		return err
	}
	return nil
}

// --- inside the window: a layer below New ---

// tenantLayer is a layer of your own, above the backend and below [cache.New].
// Embedding the ByteCache means the primitives come from the backend; the layer
// only stamps the tenant on each operation's span on the way through.
type tenantLayer struct {
	cache.ByteCache
	tenant string
}

func (l *tenantLayer) GetBytes(ctx context.Context, key string) ([]byte, error) {
	l.stamp(ctx)
	return l.ByteCache.GetBytes(ctx, key)
}

func (l *tenantLayer) SetBytes(ctx context.Context, key string, val []byte, ttlSeconds int) error {
	l.stamp(ctx)
	return l.ByteCache.SetBytes(ctx, key, val, ttlSeconds)
}

func (l *tenantLayer) Delete(ctx context.Context, key string) error {
	l.stamp(ctx)
	return l.ByteCache.Delete(ctx, key)
}

// stamp writes the layer's own field onto the operation's span — the context
// the decorator handed down for this call.
func (l *tenantLayer) stamp(ctx context.Context) {
	observability.SetSpanAttributes(ctx, attribute.String("cache.tenant", l.tenant))
}

// checkLayer proves the layer reaches the running span. The context each method
// receives is the one the observability decorator built for this call, so the
// write lands on the cache operation's own span; a wrapper around the *Cache
// New returns would find no such span at all, because the operation's context
// is handed down to the layer below and never travels back out.
//
// All three operations are exercised, so a layer that stamped only some of them
// would be caught.
func checkLayer() error {
	rec := recorder()

	layer := &tenantLayer{ByteCache: cache.NewMemory(), tenant: "acme"}
	c := cache.New(layer)

	ctx := context.Background()

	// Memory never expires; a backend of your own maps the ttl onto its own
	// clock.
	if err := c.Set(ctx, "user:42", user{Name: "Ada"}, 300); err != nil {
		return errutil.Explain(err, "set")
	}

	var got user
	if err := c.Get(ctx, "user:42", &got); err != nil {
		return errutil.Explain(err, "get hit")
	}
	if got.Name != "Ada" {
		return errutil.Explain(nil, "get hit = %q, want Ada", got.Name)
	}

	// A key never written is a miss, not a backend failure.
	var missing user
	if err := c.Get(ctx, "user:404", &missing); !errors.Is(err, cache.ErrMiss) {
		return errutil.Explain(err, "get miss should report ErrMiss")
	}

	if err := c.Delete(ctx, "user:42"); err != nil {
		return errutil.Explain(err, "delete")
	}

	// The operation's context never travels back out: after all four calls the
	// caller's own context still carries no span. That is why a wrapper around
	// the *Cache could not annotate any of them — there would be nothing to
	// write to — and why the layer below it can.
	if oteltrace.SpanFromContext(ctx).IsRecording() {
		return errutil.Explain(nil, "the operation's span leaked back to the caller")
	}

	// One span per operation, each carrying the framework's attributes and the
	// one the layer below New added.
	spans := rec.Ended()
	if len(spans) != 4 {
		return errutil.Explain(nil, "recorded %d spans, want 4", len(spans))
	}
	for _, w := range []struct{ op, key, status string }{
		{"set", "user:42", "ok"},
		{"get", "user:42", "hit"},
		{"get", "user:404", "miss"},
		{"delete", "user:42", "ok"},
	} {
		if err := checkSpan(spans, w.op, w.key, w.status); err != nil {
			return err
		}
	}
	return nil
}

// checkSpan finds the operation's span and asserts the attributes on it: the
// three the framework stamped, and the one the layer below New added.
func checkSpan(spans []sdktrace.ReadOnlySpan, op, key, status string) error {
	for _, s := range spans {
		attrs := attrsOf(s)
		if attrs["cache.operation"] != op || attrs["cache.key"] != key {
			continue
		}
		if err := wantAttr(attrs, "cache.status", status); err != nil {
			return err
		}
		if err := wantAttr(attrs, "cache.tenant", "acme"); err != nil {
			return errutil.Explain(err, "%s %q lost the layer's attribute", op, key)
		}
		return nil
	}
	return errutil.Explain(nil, "no span for %s %q", op, key)
}

// --- before the window: two OpenTelemetry hooks ---

// onlyOnStart fills in the three SpanProcessor methods the reader below does not
// implement: OnStart is the only one a reader of this kind has any use for.
type onlyOnStart struct{}

func (onlyOnStart) OnEnd(sdktrace.ReadOnlySpan)      {}
func (onlyOnStart) Shutdown(context.Context) error   { return nil }
func (onlyOnStart) ForceFlush(context.Context) error { return nil }

// checkCarrier takes the framework's path: annotate the context on the way in,
// and a reader copies what it carries onto every span started below. The
// annotation needs no span handle, so it works from a middleware, where none
// exists yet.
//
// The reader is the real one, not a copy: it lives in cloud/observability beside
// the contract, and starter-otel's NewTracerProvider registers it already, so an
// application under the framework writes none of this.
func checkCarrier() error {
	ctx := observability.WithSpanAttributes(context.Background(), attribute.String("biz.line", "orders"))
	attrs, err := observedSet(ctx, sdktrace.WithSpanProcessor(observability.SpanAttributesProcessor()))
	if err != nil {
		return err
	}
	return wantAttr(attrs, "biz.line", "orders")
}

// checkBaggage swaps the framework's carrier for OpenTelemetry's own. The hook
// is the same one — what changes is how the value travels, and what that means:
// baggage members are what a propagator puts on the wire, so these fields would
// also be handed to every downstream service. The local carrier above cannot
// do that: it is a private context key no propagator knows about.
func checkBaggage() error {
	member, err := baggage.NewMember("biz.line", "orders")
	if err != nil {
		return errutil.Explain(err, "baggage member")
	}
	bag, err := baggage.New(member)
	if err != nil {
		return errutil.Explain(err, "baggage")
	}
	ctx := baggage.ContextWithBaggage(context.Background(), bag)

	attrs, err := observedSet(ctx, sdktrace.WithSpanProcessor(baggageAttrsProcessor{}))
	if err != nil {
		return err
	}
	return wantAttr(attrs, "biz.line", "orders")
}

// baggageAttrsProcessor reads the member it cares about off the context.
type baggageAttrsProcessor struct{ onlyOnStart }

func (baggageAttrsProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if m := baggage.FromContext(parent).Member("biz.line"); m.Key() != "" {
		s.SetAttributes(attribute.String(m.Key(), m.Value()))
	}
}

// --- shared ---

// recorder installs a TracerProvider built from opts with a span recorder
// appended, and returns the recorder. Installing the global provider per call
// is enough because the cache decorator resolves its tracer per operation
// rather than caching one.
func recorder(opts ...sdktrace.TracerProviderOption) *tracetest.SpanRecorder {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(append(opts, sdktrace.WithSpanProcessor(rec))...))
	return rec
}

// observedSet runs one Set through a provider built from opts, and returns the
// attributes of the single operation span it produced.
func observedSet(ctx context.Context, opts ...sdktrace.TracerProviderOption) (map[string]string, error) {
	rec := recorder(opts...)

	c := cache.New(cache.NewMemory())
	if err := c.Set(ctx, "user:42", user{Name: "Ada"}, 0); err != nil {
		return nil, errutil.Explain(err, "set")
	}

	spans := rec.Ended()
	if len(spans) != 1 {
		return nil, errutil.Explain(nil, "recorded %d spans, want 1", len(spans))
	}
	return attrsOf(spans[0]), nil
}

// attrsOf flattens a span's attributes. Emit, not AsString: AsString renders
// any non-STRING value as "", which would make an assertion on such an
// attribute pass vacuously.
func attrsOf(s sdktrace.ReadOnlySpan) map[string]string {
	attrs := make(map[string]string, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value.Emit()
	}
	return attrs
}

// wantAttr asserts one flattened attribute against the value expected of it.
func wantAttr(attrs map[string]string, key, want string) error {
	if attrs[key] != want {
		return errutil.Explain(nil, "%s = %q, want %q", key, attrs[key], want)
	}
	return nil
}
