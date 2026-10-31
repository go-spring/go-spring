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

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/stdlib/errutil"
)

// LuohuaZone is the company's preferred data-center zone, used as the default
// for the "luohua" balancer's zone parameter. A rule may override it with the
// loose-params seam: `balancer-params: {zone: ...}`.
const LuohuaZone = "cn-luohua"

// luohuaStrategy is the bean name the company's balancer answers to: a rule
// selects it with `balancer: luohua`.
const luohuaStrategy = "luohua"

// luohuaFactory builds the company's zone-affine [loadbalance.Balancer]. Its
// only parameter is the preferred zone, read out of the selection's loose-params
// seam, so a deployment can pin a service to a different zone without a new
// strategy. The keys are this strategy's own: an unknown one is rejected here,
// never silently dropped.
func luohuaFactory(sel loadbalance.Selection) (loadbalance.Balancer, error) {
	zone := LuohuaZone
	if v, ok := sel.Params["zone"]; ok {
		zone = v
	}
	for k := range sel.Params {
		if k != "zone" {
			return nil, errutil.Explain(nil, "luohua: unknown parameter %q; only \"zone\" is read", k)
		}
	}
	return luohuaBalancer{zone: zone}, nil
}

// luohuaBalancer is the luohua load-balance policy: it prefers endpoints that
// advertise the configured zone, so in-region traffic never leaves the
// company's own data centers while a matching replica exists.
type luohuaBalancer struct {
	zone string
}

// healthy reports an endpoint as pickable: not administratively disabled and
// not drained. A zero weight is the runtime drain signal (Weight=0 → 摘流), so a
// balancer must skip it even when the pool pre-filtering did not.
func healthy(e discovery.Endpoint) bool { return !e.Disabled && e.Weight > 0 }

// Pick implements [loadbalance.Balancer].
func (b luohuaBalancer) Pick(eps []discovery.Endpoint, _ loadbalance.PickInfo) (discovery.Endpoint, error) {
	for _, e := range eps {
		if healthy(e) && e.Metadata["zone"] == b.zone {
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
	return discovery.Endpoint{}, errutil.Explain(nil, "loadbalance: luohua balancer: no endpoints")
}

// Complete implements [loadbalance.Balancer]. The policy is stateless.
func (luohuaBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
