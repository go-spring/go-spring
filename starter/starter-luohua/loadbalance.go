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
	"errors"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
)

// LuohuaZone is the company's preferred data-center zone. The luohua Balancer
// pins traffic to endpoints that advertise this zone — the fleet's internal
// services mark their instances with it, and a consumer that selects the
// "luohua" load balancer stays inside the company's own region.
const LuohuaZone = "cn-luohua"

func init() {
	// Registering a company Balancer under the fleet-standard "luohua" name makes
	// it selectable by loadbalance.New("luohua") (and by any consumer that names
	// a load-balance policy). Like the bundled p2c / least-conn / consistent-hash
	// backends this is an init-time availability registration, not a config-gated
	// activation — the balancer only governs when a pool is built with it. This
	// is the go-spring loadbalance seam's company extension point: a real luohua
	// company would hang its own routing/affinity logic here.
	loadbalance.Register("luohua", func() loadbalance.Balancer { return luohuaBalancer{} })
}

// luohuaBalancer is the luohua load-balance policy: it prefers the fleet's
// cn-luohua zone, so in-region traffic never leaves the company's own data
// centers while a cn-luohua replica exists.
type luohuaBalancer struct{}

// healthy reports an endpoint as pickable: not administratively disabled and
// not drained. A zero weight is the runtime drain signal (Weight=0 → 摘流), so a
// balancer must skip it even when the pool pre-filtering did not.
func healthy(e discovery.Endpoint) bool { return !e.Disabled && e.Weight > 0 }

// Pick implements [loadbalance.Balancer].
func (luohuaBalancer) Pick(eps []discovery.Endpoint, _ loadbalance.PickInfo) (discovery.Endpoint, error) {
	for _, e := range eps {
		if healthy(e) && e.Metadata["zone"] == LuohuaZone {
			return e, nil
		}
	}
	// No in-region replica: fall back to any healthy endpoint rather than fail.
	for _, e := range eps {
		if healthy(e) {
			return e, nil
		}
	}
	if len(eps) > 0 {
		return eps[0], nil
	}
	return discovery.Endpoint{}, errors.New("loadbalance: luohua balancer: no endpoints")
}

// Complete implements [loadbalance.Balancer]. The policy is stateless.
func (luohuaBalancer) Complete(discovery.Endpoint, error) {}
