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

package StarterRegistryEtcd

import (
	"context"
	"testing"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
)

// TestRegisterMultiInstanceLive pins the BindEach assembly through a real gs
// container against a live etcd: two blocks become two backend beans, each
// contributing one discovery.Registrar (what the starter-registry core
// collects) and one health.Indicator whose probe answers for its own cluster.
// Skips when no live etcd is reachable — the consul starter carries the
// no-docker version of the same assembly contract.
func TestRegisterMultiInstanceLive(t *testing.T) {
	addr := etcdTestAddr()
	if addr == "" {
		t.Skip("no live etcd at 127.0.0.1:2379 (or ETCD_TEST_ENDPOINTS); skipping live assembly test")
	}

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.registry.etcd.one.endpoints", addr)
		app.Property("spring.registry.etcd.two.endpoints", addr)
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
				t.Fatalf("indicator %s must probe UP against the live cluster: %v", ind.Name, err)
			}
		}
		if !names["registry-etcd:one"] || !names["registry-etcd:two"] {
			t.Fatalf("want registry-etcd:one and registry-etcd:two indicators, got %v", names)
		}
	})
}

// TestRegisterNotTriggered proves the OnProperty guard: with no
// ${spring.registry.etcd.*} entries configured, no backend or indicator beans
// register.
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
