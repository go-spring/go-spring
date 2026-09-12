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

// Package StarterGateway is a standalone API gateway starter (Server class,
// self-owned port). It brings the Spring Cloud Gateway Route/Predicate/Filter
// model to Go-Spring using idiomatic Go rather than a runtime DSL: a Predicate
// is a func(*http.Request) bool, a Filter is a func(next http.Handler)
// http.Handler, and routing is plain function composition.
//
// Routes are declared under spring.gateway.routes.<id> and are hot-reloadable:
// the route table binds them through a gs.Dync field, so any standard config
// refresh (starter-config-file volume watch, starter-config-nacos, ...) rebuilds
// the compiled table with no gateway-specific machinery. A route that fails to
// compile leaves the previous table in place, so a bad edit never takes the
// gateway down.
//
// Upstreams are either direct (http(s)://host:port) or discovery-backed
// (lb://<service>), the latter reusing cloud/discovery + cloud/loadbalance.
// Forwarding runs through cloud/governance/resilience for retry/circuit-breaking, and the
// gateway contributes /metrics to the actuator management port.
package StarterGateway

import (
	"net/http"
	"net/url"

	"go-spring.org/cloud/loadbalance"
)

// Predicate reports whether a request matches a route. It is the Go-idiomatic
// equivalent of Spring's PredicateFactory<T>: built-in constructors compile
// config literals into Predicates (see predicate.go). Multiple predicates on a
// route are combined with logical AND.
type Predicate func(*http.Request) bool

// Filter wraps a handler, exactly matching the seam used by starter-lua-filter
// (Filter.Wrap) and starter-security-jwt (Authenticator.Wrap), so those can be
// mixed in as filters with no adaptation. Filters are applied outermost-first
// in declaration order; the innermost handler is the reverse proxy.
type Filter func(next http.Handler) http.Handler

// Upstream is a route's forwarding target. Exactly one of URL or Service is set:
// URL for a direct http(s):// target, Service for an lb://<name> target resolved
// through discovery + load balancing.
type Upstream struct {
	URL       *url.URL // direct target: http(s)://host[:port]
	Service   string   // lb://<service-name>, resolved via discovery
	Discovery string   // discovery backend name; empty uses the gateway default

	// pool is the load-balancing pool for an lb:// upstream, nil for a direct
	// one. It is kept on the compiled upstream so the route table can drive it
	// from governance: the balancer strategy and outlier suspension for a route
	// come from the govern rule matching "gateway:<route-id>", applied in place
	// by [RouteTable.reconcileSelection] on every recompile and on every
	// governance push.
	pool *loadbalance.Pool
}

// Route is a fully compiled routing rule: a set of predicates (AND-combined), a
// filter chain, an upstream and the resolved forwarding handler.
type Route struct {
	ID         string
	Priority   int
	Predicates []Predicate
	Filters    []Filter
	Upstream   *Upstream

	// handler is the assembled chain (filters wrapping the proxy handler), built
	// once at compile time and reused for every matching request.
	handler http.Handler
}

// match reports whether every predicate accepts req. A route with no predicates
// matches everything (a catch-all), mirroring Spring's empty-predicate route.
func (r *Route) match(req *http.Request) bool {
	for _, p := range r.Predicates {
		if !p(req) {
			return false
		}
	}
	return true
}

// RouteRaw is the on-config shape of a route, bound from
// spring.gateway.routes.<id>. String fields are parsed by the compiler (see
// compile.go) into Predicates/Filters so binding stays flat and bind-safe (no
// deeply nested maps that trip conf binding).
type RouteRaw struct {
	// Predicate literals. Absent fields contribute no predicate.
	Path    string `value:"${predicates.path:=}"`    // ant-style, e.g. /api/orders/**
	Methods string `value:"${predicates.methods:=}"` // comma list, e.g. GET,POST
	Host    string `value:"${predicates.host:=}"`    // exact or *.suffix host
	Headers string `value:"${predicates.headers:=}"` // "K:V;K2:V2", all required
	Queries string `value:"${predicates.queries:=}"` // "k=v;k2=v2", all required
	After   string `value:"${predicates.after:=}"`   // RFC3339; match only after this time

	// Filter chain, e.g. "stripPrefix(2),addRequestHeader(X-From,gw),retry".
	Filters string `value:"${filters:=}"`

	// Priority orders route matching: larger values are matched first. Unset
	// (0) routes keep the historical id-sorted order among themselves.
	Priority int `value:"${priority:=0}"`

	Upstream struct {
		Target    string `value:"${target:=}"` // lb://name or http(s)://host:port
		Discovery string `value:"${discovery:=}"`
		// The load-balancing strategy and outlier suspension for this route's
		// lb:// upstream are NOT keys here: they are governance rules matched by
		// "gateway:<route-id>" (govern.rules[N].balancer / .outlier-threshold /
		// .outlier-suspend-for), applied to the live pool on every change.
	} `value:"${upstream}"`

	Resilience struct {
		Policy string `value:"${policy:=}"` // references spring.gateway.resilience.<name>
	} `value:"${resilience}"`
}

// policyRaw is the value type of the spring.gateway.resilience.<name> map. After
// the ExecutorFor migration only the map KEYS matter — they name the routes a
// gateway builds executors for (each resolved via resilience.ExecutorFor, policy
// driven by the governance center). The fields below are retained so existing
// spring.gateway.resilience.<name>.* config keeps binding without error, but they
// are no longer read: a route's timeout/retry/breaker now comes from the governance rules document (govern.*),
// keyed by the "gateway:<name>" label, not from this struct.
// policyRaw is the value type of the spring.gateway.resilience.<name> map.
// Only the map KEYS matter — they name the routes a gateway builds executors
// for (each resolved via resilience.ExecutorFor, policy driven by the
// governance center under the "gateway:<name>" label). The value carries no
// fields: legacy per-route policy knobs were removed when policy moved to
// the governance rules document; unknown sub-keys under spring.gateway.resilience.<name>.* are
// ignored by the driver.
type policyRaw struct{}
