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

package trace

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/propagation"
)

// namedHeaderPropagator is the shape a company's own propagator takes: it
// carries a business value (e.g. X-Tenant) in a private context key and
// extracts/injects it through a single named header. Registering it and naming
// it in the propagator spec makes it ride the fleet-wide global.
type tenantCtxKey struct{}

type namedHeaderPropagator struct{}

func (namedHeaderPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	if v, ok := ctx.Value(tenantCtxKey{}).(string); ok {
		carrier.Set("X-Tenant", v)
	}
}

func (namedHeaderPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	if v := carrier.Get("X-Tenant"); v != "" {
		ctx = context.WithValue(ctx, tenantCtxKey{}, v)
	}
	return ctx
}

func (namedHeaderPropagator) Fields() []string { return []string{"X-Tenant"} }

const tenantHeader = "x-tenant-propagator-test" // unique to avoid clashing with parallel test binaries

func TestRegisterPropagatorPanics(t *testing.T) {
	assert.Panics(t, func() { RegisterPropagator("", namedHeaderPropagator{}) })
	assert.Panics(t, func() { RegisterPropagator("tenant-dup", nil) })
	// Re-registering a built-in must panic, not silently clobber.
	assert.Panics(t, func() { RegisterPropagator("tracecontext", namedHeaderPropagator{}) })
}

// TestNewPropagatorComposesRegistered exercises the company seam: a propagator
// registered under its own name is composed into the returned text-map
// propagator, and the composite round-trips the company header alongside the
// built-ins. Registration names must stay unique per process, so this test
// registers under a scratch name it cleans up via the registry's Delete.
func TestNewPropagatorComposesRegistered(t *testing.T) {
	RegisterPropagator(tenantHeader, namedHeaderPropagator{})
	defer delete(propReg, tenantHeader)

	prop, err := NewPropagator("w3c," + tenantHeader)
	assert.NoError(t, err)
	assert.NotNil(t, prop)

	// The composite advertises both the built-in fields and the company header,
	// so the two coexist rather than one replacing the other.
	fields := prop.Fields()
	assert.Contains(t, fields, "traceparent") // built-in TraceContext
	assert.Contains(t, fields, "X-Tenant")    // company named-header

	// Inject a tenant and run it through a carrier: X-Tenant rides the wire.
	ctx := context.WithValue(context.Background(), tenantCtxKey{}, "acme")
	carrier := propagation.HeaderCarrier(map[string][]string{})
	prop.Inject(ctx, carrier)
	assert.Equal(t, "acme", carrier.Get("X-Tenant"))

	// Extract restores the tenant into a fresh context.
	got := prop.Extract(context.Background(), carrier)
	assert.Equal(t, "acme", got.Value(tenantCtxKey{}))
}

func TestNewPropagatorGrammar(t *testing.T) {
	// Empty and "w3c" default to the built-in pair.
	for _, spec := range []string{"", "w3c"} {
		prop, err := NewPropagator(spec)
		assert.NoError(t, err)
		assert.NotNil(t, prop)
	}
	// "none" leaves the global untouched.
	prop, err := NewPropagator("none")
	assert.NoError(t, err)
	assert.Nil(t, prop)
	// A single registered name resolves to that propagator.
	prop, err = NewPropagator("tracecontext")
	assert.NoError(t, err)
	assert.NotNil(t, prop)
	// An unknown name is a self-diagnosing error.
	_, err = NewPropagator("w3c,no-such-propagator")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown propagator")
	assert.True(t, strings.Contains(err.Error(), "tracecontext"))
}
