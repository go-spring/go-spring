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

package governance

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/inbound/observe"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestInboundOver_InjectsFaultThroughGuard pins the single inbound seam: the
// fault authority is folded into the guard as its innermost stage, so a server
// adapter needs one Inbound and cannot double-fire.
func TestInboundOver_InjectsFaultThroughGuard(t *testing.T) {
	inj := serverfault.NewManager()
	inj.Refresh(fault.Config{Rules: []fault.Rule{{Name: "all", Service: "gin", Rate: 1, Class: fault.ClassTransient, Error: "generic"}}})
	gm := admission.NewManager()
	defer gm.Close()

	in, err := InboundOver("gin", "gin:test", gm, inj)
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

// TestInboundOver_NoInjectorIsGovernanceOnly pins that omitting the fault
// authority leaves a plain, admitting inbound: fault is optional, not a second
// always-on path.
func TestInboundOver_NoInjectorIsGovernanceOnly(t *testing.T) {
	gm := admission.NewManager()
	defer gm.Close()

	in, err := InboundOver("gin", "gin:test", gm)
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

// TestInboundOver_TagsInjectedFaultOnSpan pins the inbound half of the
// "an injected fault is never read as a real failure" contract: the span carries
// the same attributes the outbound edge writes, naming the experiments that
// fired. Without it an inbound fire is indistinguishable from a genuine 5xx.
func TestInboundOver_TagsInjectedFaultOnSpan(t *testing.T) {
	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prev)
		_ = tp.Shutdown(context.Background())
	})

	inj := serverfault.NewManager()
	inj.Refresh(fault.Config{Rules: []fault.Rule{{
		Name: "all", Service: "gin", Rate: 1, Class: fault.ClassTransient, Error: "generic",
	}}})
	gm := admission.NewManager()
	defer gm.Close()

	in, err := InboundOver("gin", "gin:test", gm, inj)
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
	if got := attrs["resilience.injected"].AsString(); got != "generic" {
		t.Fatalf("resilience.injected = %q, want generic", got)
	}
	if got := attrs["resilience.injected.rule"].AsString(); got != "all" {
		t.Fatalf("resilience.injected.rule = %q, want all", got)
	}
}
