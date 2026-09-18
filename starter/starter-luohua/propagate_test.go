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
	"bytes"
	"context"
	"strings"
	"testing"

	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/propagation"
)

// luohuaTestTag is registered at package level: log.RegisterTag panics once the
// log system has been refreshed, so it has to happen before any test runs.
var luohuaTestTag = log.RegisterTag("_luohua_test")

// logLine renders one log event with ctx and returns the line, so a test can
// assert on the fields that reached the log through the context.
func logLine(ctx context.Context) string {
	prev := log.Stdout
	buf := bytes.NewBuffer(nil)
	log.Stdout = buf
	defer func() { log.Stdout = prev }()
	log.Info(ctx, luohuaTestTag, log.Msgf("probe"))
	return buf.String()
}

// carriedAttr reads one span attribute off ctx, as the SpanProcessor in
// starter-otel would see it.
func carriedAttr(ctx context.Context, key string) (string, bool) {
	for _, a := range observability.ContextAttributes(ctx) {
		if string(a.Key) == key {
			return a.Value.Emit(), true
		}
	}
	return "", false
}

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

// TestApplyObservabilitySurfacesCarriedFields verifies luohua's configured
// business fields reach BOTH signals: the log line and the span attributes. It
// is what replaced the log.FieldsFromContext hook -- one push covers both,
// instead of a hook that covered logs only and occupied the single global slot.
func TestApplyObservabilitySurfacesCarriedFields(t *testing.T) {
	defer setObservabilityFields(nil)
	if err := applyObservability(ObservabilityConfig{Fields: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyObservability: %v", err)
	}

	ctx := annotate(putCarriedHeader(context.Background(), "X-Tenant", "acme"))

	if line := logLine(ctx); !strings.Contains(line, "X-Tenant=acme") {
		t.Fatalf("log line %q does not carry X-Tenant=acme", line)
	}
	if v, ok := carriedAttr(ctx, "X-Tenant"); !ok || v != "acme" {
		t.Fatalf("span attribute X-Tenant = %q (present=%v), want acme", v, ok)
	}
}

// TestAnnotateLeavesUntouchedContextsAlone proves annotate is inert when it has
// nothing configured or nothing carried -- it must not wrap the context, and
// must not invent fields.
func TestAnnotateLeavesUntouchedContextsAlone(t *testing.T) {
	setObservabilityFields(nil)
	defer setObservabilityFields(nil)

	plain := context.Background()
	if got := annotate(plain); got != plain {
		t.Fatal("annotate with nothing configured should return the context unchanged")
	}

	if err := applyObservability(ObservabilityConfig{Fields: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyObservability: %v", err)
	}
	// Configured, but the value never arrived: nothing to attach.
	if got := annotate(plain); got != plain {
		t.Fatal("annotate with nothing carried should return the context unchanged")
	}
}

// TestApplyObservabilityNoFields proves an empty field list leaves both signals
// untouched, so a deployment that configures nothing gets no extra telemetry.
func TestApplyObservabilityNoFields(t *testing.T) {
	defer setObservabilityFields(nil)
	if err := applyObservability(ObservabilityConfig{}); err != nil {
		t.Fatalf("applyObservability: %v", err)
	}
	ctx := putCarriedHeader(context.Background(), "X-Tenant", "acme")
	if got := annotate(ctx); got != ctx {
		t.Fatal("annotate with no fields configured should return the context unchanged")
	}
}
