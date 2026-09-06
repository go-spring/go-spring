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
	"testing"

	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/log"
	"go-spring.org/starter-otel/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// TestPropagationRidesOtelGlobal proves the axis-1 chain end to end, offline:
// with luohua's named-header propagator composed into the OTel global (exactly
// what starter-otel setupTrace does for propagator=w3c,luohua) and the log
// hook installed, a request carrying X-Tenant is extracted by the same global
// the gin/echo/grpc transports call — then luohua's log hook prints the tenant.
func TestPropagationRidesOtelGlobal(t *testing.T) {
	// Preserve process globals and restore them so this test is hermetic.
	prevLogHook := log.FieldsFromContext
	prevProp := otel.GetTextMapPropagator()
	prevTrafficHeader, prevMeta := canonical.HeaderLoadTest, canonical.MetaKeyLoadTest
	defer func() {
		log.FieldsFromContext = prevLogHook
		otel.SetTextMapPropagator(prevProp)
		canonical.HeaderLoadTest, canonical.MetaKeyLoadTest = prevTrafficHeader, prevMeta
		setCarriedHeaders(nil)
	}()

	// Arm luohua: the wire vocabulary (X-Tenant carried) + observability (it is
	// also printed on logs). No traffic-header override here.
	if err := applyPropagate(PropagateConfig{Headers: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyPropagate: %v", err)
	}
	if err := applyObservability(ObservabilityConfig{Fields: []string{"X-Tenant"}}); err != nil {
		t.Fatalf("applyObservability: %v", err)
	}

	// Compose luohua into the global exactly as starter-otel would for
	// propagator=w3c,luohua.
	prop, err := trace.NewPropagator("w3c," + propagatorName)
	if err != nil {
		t.Fatalf("NewPropagator: %v", err)
	}
	otel.SetTextMapPropagator(prop)

	// A transport (gin/echo/grpc) extracts the inbound request through the global.
	inbound := propagation.HeaderCarrier(map[string][]string{"X-Tenant": {"acme"}})
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), inbound)
	if got := carryHeader(ctx, "X-Tenant"); got != "acme" {
		t.Fatalf("inbound X-Tenant = %q, want acme", got)
	}

	// The observability log hook surfaces it as a log field.
	fields := log.FieldsFromContext(ctx)
	saw := false
	for _, f := range fields {
		if f.Key == "X-Tenant" {
			saw = true
		}
	}
	if !saw {
		t.Fatal("log hook did not surface the carried X-Tenant field")
	}
}
