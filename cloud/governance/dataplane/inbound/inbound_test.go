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

package inbound

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestOver_InjectsFaultThroughGuard pins the single inbound seam: the
// fault authority is folded into the guard as its innermost stage, so a server
// adapter needs one Inbound and cannot double-fire.
func TestOver_InjectsFaultThroughGuard(t *testing.T) {
	inj := serverfault.NewManager()
	inj.Refresh(serverfault.ServerConfig{Rules: []serverfault.ServerRule{{
		RuleCore: fault.RuleCore{Name: "all", Service: "gin", Rate: 1},
		Status:   503,
	}}})
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Close()

	in, err := Over("gin", "gin:test", gm, nil, inj)
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	err = in.Run(context.Background(), observe.ServerOperation{Action: "GET /x"},
		func(context.Context) error {
			ran = true
			return nil
		})

	var ref *Refusal
	if !errors.As(err, &ref) {
		t.Fatalf("want an injected refusal, got %v", err)
	}
	if ref.Verdict() != admission.VerdictFaultInjected {
		t.Fatalf("want fault injected, got %v", ref.Verdict())
	}
	if ran {
		t.Fatal("an injected fault must not reach the handler")
	}
}

// TestOver_NoInjectorIsGovernanceOnly pins that omitting the fault
// authority leaves a plain, admitting inbound: fault is optional, not a second
// always-on path.
func TestOver_NoInjectorIsGovernanceOnly(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Close()

	in, err := Over("gin", "gin:test", gm, nil)
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	err = in.Run(context.Background(),
		observe.ServerOperation{Action: "GET /x"}, func(context.Context) error {
			ran = true
			return nil
		})
	if err != nil {
		t.Fatalf("want the request admitted, got %v", err)
	}
	if !ran {
		t.Fatal("the handler did not run")
	}
}

// TestOver_TagsInjectedFaultOnSpan pins the inbound half of the
// "an injected fault is never read as a real failure" contract: the span carries
// the same attributes the outbound edge writes, naming the experiments that
// fired. Without it an inbound fire is indistinguishable from a genuine 5xx.
func TestOver_TagsInjectedFaultOnSpan(t *testing.T) {
	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	inj := serverfault.NewManager()
	inj.Refresh(serverfault.ServerConfig{Rules: []serverfault.ServerRule{{
		RuleCore: fault.RuleCore{Name: "all", Service: "gin", Rate: 1},
		Status:   503,
	}}})
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Close()

	in, err := Over("gin", "gin:test", gm, nil, inj)
	if err != nil {
		t.Fatal(err)
	}
	_ = in.Run(context.Background(), observe.ServerOperation{Action: "GET /x"},
		func(context.Context) error { return nil })

	spans := spanExp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("want one server span, got %d", len(spans))
	}
	attrs := make(map[string]attribute.Value, len(spans[0].Attributes))
	for _, kv := range spans[0].Attributes {
		attrs[string(kv.Key)] = kv.Value
	}
	if got := attrs["resilience.injected"].AsString(); got != "response" {
		t.Fatalf("resilience.injected = %q, want response", got)
	}
	if got := attrs["resilience.injected.status"].AsInt64(); got != 503 {
		t.Fatalf("resilience.injected.status = %d, want 503", got)
	}
	if got := attrs["resilience.injected.rule"].AsString(); got != "all" {
		t.Fatalf("resilience.injected.rule = %q, want all", got)
	}
}

// fakeObservation is a minimal Observation for a fake emitter.
type fakeObservation struct{}

func (fakeObservation) Response() *observe.Response { return &observe.Response{} }
func (fakeObservation) MarkInjected(fault.Result)   {}

// fakeMetrics records that the handle drove the injected metric seam.
type fakeMetrics struct{ requests int }

func (m *fakeMetrics) Active(context.Context, observe.Request, int64) {}

func (m *fakeMetrics) Request(context.Context, observe.Request) { m.requests++ }

func (m *fakeMetrics) Fault(context.Context, observe.Request, observe.Fault) {}

// TestOver_UsesInjectedMetrics proves the inbound handle reports through the
// injected metric seam instead of the OTel one.
func TestOver_UsesInjectedMetrics(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	gm, err := admission.NewManager(factories)
	if err != nil {
		t.Fatal(err)
	}
	defer gm.Close()

	m := &fakeMetrics{}
	in, err := Over("sys", "svc", gm, m)
	if err != nil {
		t.Fatal(err)
	}
	ran := false
	err = in.Run(context.Background(), observe.ServerOperation{Action: "GET /x"}, func(context.Context) error {
		ran = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ran || m.requests != 1 {
		t.Fatalf("expected the handler and the injected metrics to run: handler=%v metrics=%d", ran, m.requests)
	}
}
