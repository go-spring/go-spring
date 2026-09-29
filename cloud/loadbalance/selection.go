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

// selection.go is the endpoint-selection half of a governance rule: the flat,
// directly bindable policy the loadbalance pool applies. Like
// resilience.ClientPolicy it is ONE definition — the bindable form IS the runtime
// form; there is no separate binding twin.

package loadbalance

import "time"

// Selection is the resolved endpoint-selection policy: the strategy (with its
// construction parameters, flat) and the outlier-suspension thresholds. A
// balancer's construction parameters take effect together with a Balancer name
// change — a rule that only retunes thresholds leaves the current strategy
// untouched.
type Selection struct {
	// Balancer is the strategy name to select endpoints with (e.g.
	// "least_conn"). Empty means "leave the pool's current strategy alone".
	Balancer string `value:"${balancer:=}"`

	// BalancerReplicas is the number of virtual nodes per endpoint for
	// consistent-hash strategies (0 = the strategy default).
	BalancerReplicas int `value:"${balancer-replicas:=0}"`

	// BalancerZoneKey is the endpoint metadata key a zone-aware strategy reads
	// for locality (empty = the loadbalance default).
	BalancerZoneKey string `value:"${balancer-zone-key:=}"`

	// BalancerDelegate names the strategy a zone-aware balancer delegates the
	// final choice to (empty = round-robin).
	BalancerDelegate string `value:"${balancer-delegate:=}"`

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int `value:"${outlier-threshold:=0}"`

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration `value:"${outlier-suspend-for:=0}"`
}

// BalancerConfig translates the flat selection knobs into the [Config] a
// strategy factory consumes. It is the only place the flat names meet the
// factory's parameter struct.
func (s Selection) BalancerConfig() Config {
	return Config{Replicas: s.BalancerReplicas, ZoneKey: s.BalancerZoneKey, Delegate: s.BalancerDelegate}
}

// Selection returns the endpoint-selection policy currently in force.
//
// The strategy name is the last one *accepted*: applying an empty name leaves
// it untouched, and an unknown name is ignored, so the name keeps describing
// the strategy actually in force. The balancer handed to [NewPool] is not
// recorded here — it had no name to record. It is an inspection and test
// helper.
func (p *Pool) Selection() Selection {
	if s := p.sel.Load(); s != nil {
		return *s
	}
	return Selection{}
}
