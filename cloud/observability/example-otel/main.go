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

// Command example-otel shows where an attribute can reach an operation's span,
// and asserts each route lands.
//
// A span is started before the work and settled after it, so there are two
// moments to write into it. Inside that window only code holding the
// span-carrying context can write, and a layer of your own called from that
// window holds it. Before the window there is no span to hold, so the write goes
// through an OpenTelemetry hook instead; this demo runs two of those, each under
// its own provider:
//
//	inside  a layer under the span                         the span handle
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
	"fmt"
	"os"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// componentName is the instrumentation componentName the demo's span reports under.
const componentName = "go-spring.org/cloud/observability/example"

// attrTenant is the attribute the layer inside the window writes.
const attrTenant = "biz.tenant"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("observability example-otel ok")
}

func run() error {
	if err := checkLayer(); err != nil {
		return err
	}
	if err := checkCarrier(); err != nil {
		return err
	}
	return checkBaggage()
}

// --- inside the window: a layer under the span ---

// tenantLayer stands for a layer of your own inside the call. It holds the
// context it was handed and writes to the span that context carries.
type tenantLayer struct{ tenant string }

func (l tenantLayer) stamp(ctx context.Context) {
	observability.SetSpanAttributes(ctx, attribute.String(attrTenant, l.tenant))
}

// checkLayer proves the layer reaches the running span: the context it receives
// is the one the span was started with, so the write lands on it. Code ABOVE the
// instrumentation point would find no such span at all — the span's context is
// handed down and never travels back out.
//
// Several calls are exercised, so a layer that stamped only some of them would
// be caught.
func checkLayer() error {
	calls := []func(ctx context.Context) error{}
	layer := tenantLayer{tenant: "acme"}
	for range 4 {
		calls = append(calls, func(ctx context.Context) error {
			layer.stamp(ctx)
			return nil
		})
	}

	for i, fn := range calls {
		attrs, err := call(context.Background(), fn)
		if err != nil {
			return errutil.Explain(err, "call %d", i)
		}
		if err := wantAttr(attrs, attrTenant, "acme"); err != nil {
			return errutil.Explain(err, "call %d lost the layer's attribute", i)
		}
	}

	// The span's context never travels back out: the caller's own context still
	// carries no span once the calls return.
	if oteltrace.SpanFromContext(context.Background()).IsRecording() {
		return errutil.Explain(nil, "the operation's span leaked back to the caller")
	}
	return nil
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
	attrs, err := call(ctx, nop, sdktrace.WithSpanProcessor(observability.SpanAttributesProcessor()))
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

	attrs, err := call(ctx, nop, sdktrace.WithSpanProcessor(baggageAttrsProcessor{}))
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

// nop is the call the hook routes exercise: they are about what the span ends up
// carrying, not about what the call does.
func nop(context.Context) error { return nil }

// call opens a span, runs fn under it, and returns the attributes the span
// carried when it ended. It is the shape a component uses when it instruments
// its own calls — start, run, settle — and the one this package's carrier is
// written for: whatever the context already carries is applied to the span as it
// starts ([observability.SpanAttributesProcessor]).
//
// Installing the global provider per call is enough because the tracer is
// resolved per call rather than cached.
func call(ctx context.Context, fn func(context.Context) error, opts ...sdktrace.TracerProviderOption) (map[string]string, error) {
	rec := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(append(opts, sdktrace.WithSpanProcessor(rec))...))

	ctx, span := otel.Tracer(componentName).Start(ctx, "op")
	err := fn(ctx)
	span.End()
	if err != nil {
		return nil, err
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
