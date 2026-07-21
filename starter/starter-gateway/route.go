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
// (lb://<service>), the latter reusing stdlib/discovery + stdlib/loadbalance.
// Forwarding runs through stdlib/resilience for retry/circuit-breaking, and the
// gateway contributes /metrics to the actuator management port.
package StarterGateway

import (
	"net/http"
	"net/url"
	"time"
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
	Balancer  string   // loadbalance strategy name; empty defaults to round_robin
	Discovery string   // discovery backend name; empty uses the gateway default
}

// Route is a fully compiled routing rule: a set of predicates (AND-combined), a
// filter chain, an upstream and the resolved forwarding handler.
type Route struct {
	ID         string
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

	Upstream struct {
		Target    string `value:"${target:=}"` // lb://name or http(s)://host:port
		Balancer  string `value:"${balancer:=round_robin}"`
		Discovery string `value:"${discovery:=}"`
	} `value:"${upstream}"`

	Resilience struct {
		Policy string `value:"${policy:=}"` // references spring.gateway.resilience.<name>
	} `value:"${resilience}"`
}

// policyRaw is a named resilience policy under spring.gateway.resilience.<name>.
// It mirrors resilience.Policy so routes can share pooled breaker/limiter state
// by referencing a policy name.
type policyRaw struct {
	RateLimit      float64       `value:"${rateLimit:=0}"`
	Burst          int           `value:"${burst:=0}"`
	ErrorThreshold int           `value:"${errorThreshold:=0}"`
	OpenDuration   time.Duration `value:"${openDuration:=0}"`
	MaxConcurrent  int           `value:"${maxConcurrent:=0}"`
	MaxRetries     int           `value:"${maxRetries:=0}"`
	Timeout        time.Duration `value:"${timeout:=0}"`
}
