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

package luohua

import (
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/propagation"
)

// TestPropagateRoundTrip verifies the named-header propagator carries a
// configured business header (X-Tenant) across a text-map carrier and back
// into the context.
func TestPropagateRoundTrip(t *testing.T) {
	if err := applyPropagate(PropagateConfig{Headers: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyPropagate: %v", err)
	}

	ctx := putCarriedHeader(context.Background(), "X-Tenant", "acme")
	carrier := propagation.HeaderCarrier(map[string][]string{})
	luohuaPropagator{}.Inject(ctx, carrier)
	if got := carrier.Get("X-Tenant"); got != "acme" {
		t.Fatalf("Inject: header = %q, want acme", got)
	}

	got := luohuaPropagator{}.Extract(context.Background(), carrier)
	if v := carryHeader(got, "X-Tenant"); v != "acme" {
		t.Fatalf("Extract: ctx tenant = %q, want acme", v)
	}

	fields := luohuaPropagator{}.Fields()
	if len(fields) != 1 || fields[0] != "X-Tenant" {
		t.Fatalf("Fields() = %v, want [X-Tenant]", fields)
	}
}

// TestApplyPropagateOverridesTrafficHeader verifies the G1 seam: configuring a
// load-test header re-binds traffic detection onto luohua's convention.
func TestApplyPropagateOverridesTrafficHeader(t *testing.T) {
	const hdr, meta = "X-Luohua-Load", "x-luohua-load"
	defer func() {
		canonical.HeaderLoadTest, canonical.MetaKeyLoadTest = "X-LoadTest", "x-loadtest"
	}()

	if err := applyPropagate(PropagateConfig{LoadTestHeader: hdr}); err != nil {
		t.Fatalf("applyPropagate: %v", err)
	}
	if canonical.HeaderLoadTest != hdr {
		t.Fatalf("canonical.HeaderLoadTest = %q, want %q", canonical.HeaderLoadTest, hdr)
	}
	if canonical.MetaKeyLoadTest != meta {
		t.Fatalf("canonical.MetaKeyLoadTest = %q, want %q", canonical.MetaKeyLoadTest, meta)
	}
}

// TestApplyObservabilityLogHook verifies the log context hook prints luohua's
// carried business fields on top of any existing hook.
func TestApplyObservabilityLogHook(t *testing.T) {
	prev := log.FieldsFromContext
	defer func() { log.FieldsFromContext = prev }()

	if err := applyObservability(ObservabilityConfig{Fields: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyObservability: %v", err)
	}
	if log.FieldsFromContext == nil {
		t.Fatal("log.FieldsFromContext was not installed")
	}

	ctx := putCarriedHeader(context.Background(), "X-Tenant", "acme")
	fields := log.FieldsFromContext(ctx)
	found := false
	for _, f := range fields {
		if f.Key == "X-Tenant" {
			found = true
		}
	}
	if !found {
		var keys []string
		for _, f := range fields {
			keys = append(keys, f.Key)
		}
		t.Fatalf("log fields %v do not include X-Tenant", strings.Join(keys, ","))
	}
}
