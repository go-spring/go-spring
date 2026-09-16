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

package StarterRegistryConsul

import (
	"context"
	"net/http/httptest"
	"testing"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
)

// TestRegisterMultiInstance pins the BindEach assembly through a real gs
// container: two blocks under ${spring.registry.consul} become two backend
// beans, each contributing one discovery.Registrar (what the starter-registry
// core collects) and one health.Indicator. The two fake agents stand in for
// two Consul clusters, so the whole construction path — client, startup
// probe, both halves — runs without docker.
func TestRegisterMultiInstance(t *testing.T) {
	srvA := httptest.NewServer(&fakeConsul{})
	defer srvA.Close()
	srvB := httptest.NewServer(&fakeConsul{})
	defer srvB.Close()

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.registry.consul.one.address", srvA.URL)
		app.Property("spring.registry.consul.two.address", srvB.URL)
	}).RunTest(t, func(s *struct {
		Regs []discovery.Registrar `autowire:""`
		Inds []*health.Indicator   `autowire:""`
	}) {
		if len(s.Regs) != 2 {
			t.Fatalf("want 2 registrar beans (one per block), got %d", len(s.Regs))
		}

		names := map[string]bool{}
		for _, ind := range s.Inds {
			names[ind.Name] = true
			if err := ind.Probe(context.Background()); err != nil {
				t.Fatalf("indicator %s must probe UP against its fake agent: %v", ind.Name, err)
			}
		}
		if !names["registry-consul:one"] || !names["registry-consul:two"] {
			t.Fatalf("want registry-consul:one and registry-consul:two indicators, got %v", names)
		}
	})
}

// TestRegisterNotTriggered proves the OnProperty guard: with no
// ${spring.registry.consul.*} entries configured, no backend or indicator
// beans register.
func TestRegisterNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Regs []discovery.Registrar `autowire:""`
		Inds []*health.Indicator   `autowire:""`
	}) {
		if len(s.Regs) != 0 {
			t.Fatalf("no registrar beans should register without config, got %d", len(s.Regs))
		}
		if len(s.Inds) != 0 {
			t.Fatalf("no indicators should register without config, got %d", len(s.Inds))
		}
	})
}
