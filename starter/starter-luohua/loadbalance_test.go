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
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
)

// TestLoadBalanceCompanyRegisteredAndZoneAffine locks the luohua loadbalance
// capability: the company contributes a "luohua" Factory (the go-spring
// loadbalance extension point) with a real, observable company policy — it pins
// traffic to the configured zone, defaulting to the fleet's cn-luohua. A
// company owns its routing/affinity decision here, and a deployment can retune
// the zone through the strategy's own parameter.
func TestLoadBalanceCompanyRegisteredAndZoneAffine(t *testing.T) {
	dir, err := loadbalance.NewDirectory(map[string]loadbalance.Factory{luohuaStrategy: luohuaFactory{}})
	if err != nil {
		t.Fatalf("luohua factory not accepted: %v", err)
	}
	b, err := dir.Build(luohuaStrategy, loadbalance.NewParams(nil))
	if err != nil {
		t.Fatalf("luohua balancer not built: %v", err)
	}
	if _, ok := b.(luohuaBalancer); !ok {
		t.Fatalf("balancer is %T, want luohuaBalancer", b)
	}

	// The zone parameter overrides the default, so one strategy pins traffic to
	// whichever zone a rule names.
	other, err := dir.Build(luohuaStrategy, loadbalance.NewParams(map[string]string{"zone": "cn-other-2"}))
	if err != nil {
		t.Fatalf("luohua balancer with a zone parameter: %v", err)
	}
	zoned := []discovery.Endpoint{
		{Addr: "10.0.0.1:8080", Weight: 100, Metadata: map[string]string{"zone": "cn-other"}},
		{Addr: "10.0.0.2:8080", Weight: 100, Metadata: map[string]string{"zone": "cn-other-2"}},
	}
	if got, err := other.Pick(zoned, loadbalance.PickInfo{}); err != nil || got.Addr != "10.0.0.2:8080" {
		t.Fatalf("Pick with zone=cn-other-2: got %s err=%v, want 10.0.0.2", got.Addr, err)
	}

	// An unknown parameter is rejected at construction, not silently dropped.
	if _, err := dir.Build(luohuaStrategy, loadbalance.NewParams(map[string]string{"zoen": "typo"})); err == nil {
		t.Fatal("a misspelled parameter must fail construction")
	}

	eps := []discovery.Endpoint{
		{Addr: "10.0.0.1:8080", Weight: 100, Metadata: map[string]string{"zone": "cn-other"}},
		{Addr: "10.0.0.2:8080", Weight: 100, Metadata: map[string]string{"zone": LuohuaZone}},
	}
	got, err := b.Pick(eps, loadbalance.PickInfo{})
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	if got.Addr != "10.0.0.2:8080" {
		t.Fatalf("Pick with a cn-luohua replica: got %s, want the in-region 10.0.0.2", got.Addr)
	}

	// No in-region replica: fall back to a healthy endpoint, never an error.
	only := []discovery.Endpoint{{Addr: "10.0.0.3:8080", Weight: 100, Metadata: map[string]string{"zone": "cn-other"}}}
	got, err = b.Pick(only, loadbalance.PickInfo{})
	if err != nil || got.Addr != "10.0.0.3:8080" {
		t.Fatalf("Pick without a cn-luohua replica: got %s err=%v, want healthy 10.0.0.3", got.Addr, err)
	}

	// Disabled (zero-weight drain) replicas are skipped even in-zone.
	dis := []discovery.Endpoint{
		{Addr: "10.0.0.4:8080", Weight: 0, Metadata: map[string]string{"zone": LuohuaZone}},
		{Addr: "10.0.0.5:8080", Weight: 100, Metadata: map[string]string{"zone": "cn-other"}},
	}
	got, err = b.Pick(dis, loadbalance.PickInfo{})
	if err != nil || got.Addr != "10.0.0.5:8080" {
		t.Fatalf("Pick skipping a drained in-zone replica: got %s err=%v, want 10.0.0.5", got.Addr, err)
	}
}
