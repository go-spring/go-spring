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
// resilience.Policy it is ONE definition — the bindable form IS the runtime
// form; there is no separate binding twin.

package loadbalance

import (
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/matcher"
)

// Selection is the resolved endpoint-selection policy: the strategy (by name),
// the strategy's own parameters, and the outlier-suspension thresholds.
//
// It is the COMPLETE description of how a label selects endpoints: a resolved
// selection leaves nothing to the configuration it replaces (see [Chain]).
type Selection struct {
	// Balancer is the strategy name to select endpoints with (e.g.
	// "least_conn"). It names an entry in the process's strategy registry: a built-in
	// strategy, or one a deployment contributed as a [Factory] bean. Empty means the
	// base strategy, round-robin — there being no "current strategy" to leave alone,
	// a rebuild having nothing to inherit.
	Balancer string `value:"${balancer:=}"`

	// Params carries the strategy's own construction parameters — flat, and
	// opaque to this package: the strategy reads the keys it owns and rejects
	// the rest (see [Params]). It binds from the `balancer-params` sub-map of a
	// rule, so a rule reads:
	//
	//	balancer: consistent_hash
	//	balancer-params:
	//	  replicas: 200
	//
	// A strategy with new parameters needs no change here.
	Params map[string]string `value:"${balancer-params:=}"`

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int `value:"${outlier-threshold:=0}" expr:"$ >= 0"`

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration `value:"${outlier-suspend-for:=0}" expr:"int($) >= 0"`

	// Selector narrows the endpoint set this label may choose from, before the
	// strategy picks — a conjunction over the endpoint's metadata, see
	// [matcher.Selector]. It is the DYNAMIC half of endpoint narrowing: the
	// static half is the discovery query's selector, fixed by the client's own
	// configuration, and when both are set both must hold. Because
	// it rides the selection document, a pushed rule can move traffic between
	// versions or zones without a redeploy, and a change rebuilds the chain's
	// generation like any other selection change.
	//
	// Empty means no narrowing. A selector that matches nothing leaves the pool
	// with no candidates, which surfaces as [ErrNoAvailable] — there is
	// deliberately no implicit fallback, because silently falling back would
	// make a narrow rule (a canary, say) widen to every instance without anyone
	// being told.
	Selector matcher.Selector `value:"${selector:=}"`
}

// Selection returns the selection the pool was BUILT from.
//
// A pool never changes its strategy after construction, so this is what the
// pool is — not what some later document said. A name that failed to build is
// not recorded (the last good generation stays in force instead, see [Chain]),
// and a pool built from a bare [Balancer] records nothing — it had no name to
// record. It is an inspection and test helper.
func (p *Pool) Selection() Selection {
	if s := p.sel.Load(); s != nil {
		return *s
	}
	return Selection{}
}
