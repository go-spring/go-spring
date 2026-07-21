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

package StarterGateway

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/spring/gs"
)

// FilterWrapper is the seam a bean-backed filter (jwt-auth, lua) satisfies:
// exactly the Wrap method already exposed by starter-security-jwt's
// Authenticator and starter-lua-filter's Filter. An application exports such a
// bean as gateway.FilterWrapper with a name and references it by that name in a
// route's filter list (e.g. filters=jwt-auth(myAuth)). The gateway stays fully
// decoupled from those modules — it only depends on this one-method interface.
type FilterWrapper interface {
	Wrap(next http.Handler) http.Handler
}

// gatewayConfig binds the gateway's routing config under ${spring.gateway}.
// Routes is dynamic: any standard config refresh rebinds it, and the route table
// recompiles on the next request (see RouteTable.current).
type gatewayConfig struct {
	Routes     gs.Dync[map[string]RouteRaw] `value:"${routes:=}"`
	Resilience map[string]policyRaw         `value:"${resilience:=}"`
	Discovery  string                       `value:"${discovery:=}"`
}

// RouteTable holds the compiled routes and rebuilds them when the bound routes
// map changes. Matching reads a lock-free atomic snapshot; recompilation only
// happens when the underlying Dync map is swapped by a config refresh.
type RouteTable struct {
	ctx     context.Context
	metrics *Metrics

	// Cfg is bound from ${spring.gateway} by field injection after construction.
	// Its Routes field is a gs.Dync map, so a config refresh swaps the underlying
	// map and current() recompiles on the next request.
	Cfg gatewayConfig `value:"${spring.gateway}"`

	// Wrappers holds bean-backed filters (jwt-auth, lua) collected by name. It is
	// populated by field injection after the constructor returns, so route
	// compilation must be deferred until warmup() runs (see GatewayServer.Run).
	Wrappers map[string]FilterWrapper `autowire:"?"`

	compiled atomic.Pointer[[]*Route]

	mu        sync.Mutex // guards recompilation and lastPtr
	lastPtr   uintptr    // identity of the last-compiled routes map
	discovery string

	dialerMu sync.Mutex
	dialers  map[string]*discovery.LiveDialer

	// execs pools resilience executors by policy name so routes sharing a policy
	// share breaker/limiter state. Rebuilt on each recompile.
	execs map[string]resilience.Executor
}

// newRouteTable builds the table. Config (Cfg) and bean-backed filters (Wrappers)
// are populated by field injection after the constructor returns, so route
// compilation is deferred to warmup() — called from GatewayServer.Run — where a
// bad initial config fails startup.
func newRouteTable(cp *gs.ContextProvider, m *Metrics) *RouteTable {
	return &RouteTable{
		ctx:     cp.Context,
		metrics: m,
		dialers: map[string]*discovery.LiveDialer{},
	}
}

// warmup compiles the initial route table. It runs after the Wrappers field has
// been injected, so bean-backed filters resolve; any error fails startup.
func (t *RouteTable) warmup() error {
	t.discovery = t.Cfg.Discovery
	return t.recompile(t.Cfg.Routes.Value())
}

// current returns the live compiled routes, recompiling first if a config
// refresh has swapped in a new routes map. The cheap fast path is a single
// atomic load plus a pointer compare; recompilation is rare (only on refresh).
func (t *RouteTable) current() []*Route {
	raw := t.Cfg.Routes.Value()
	if reflect.ValueOf(raw).Pointer() != atomic.LoadUintptr(&t.lastPtr) {
		if err := t.recompile(raw); err != nil {
			// Keep serving the previous table: a bad hot edit must never take the
			// gateway down. Surface it loudly via log + metric.
			t.metrics.recordReloadError()
			log.Errorf(t.ctx, log.TagAppDef, "gateway: route reload failed, keeping previous table: %v", err)
			// Adopt the pointer so we do not retry the same broken map every request.
			atomic.StoreUintptr(&t.lastPtr, reflect.ValueOf(raw).Pointer())
		}
	}
	if p := t.compiled.Load(); p != nil {
		return *p
	}
	return nil
}

// Match returns the first route whose predicates all accept req, or nil.
func (t *RouteTable) Match(req *http.Request) *Route {
	for _, r := range t.current() {
		if r.match(req) {
			return r
		}
	}
	return nil
}

// recompile builds a fresh compiled route slice from raw and atomically swaps it
// in. On any error it leaves the current table untouched and returns the error.
// Routes are ordered by id for deterministic matching.
func (t *RouteTable) recompile(raw map[string]RouteRaw) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	execs, err := t.buildExecutors()
	if err != nil {
		return err
	}

	ids := make([]string, 0, len(raw))
	for id := range raw {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	routes := make([]*Route, 0, len(ids))
	for _, id := range ids {
		rt, err := t.compileRoute(id, raw[id], execs)
		if err != nil {
			return fmt.Errorf("route %q: %w", id, err)
		}
		routes = append(routes, rt)
	}

	t.compiled.Store(&routes)
	t.execs = execs
	atomic.StoreUintptr(&t.lastPtr, reflect.ValueOf(raw).Pointer())
	return nil
}

// buildExecutors turns each named resilience policy into an Executor via the
// builtin driver. Routes reference these by name so they share breaker state.
func (t *RouteTable) buildExecutors() (map[string]resilience.Executor, error) {
	if len(t.Cfg.Resilience) == 0 {
		return nil, nil
	}
	driver, err := resilience.MustGetDriver("default")
	if err != nil {
		return nil, err
	}
	out := make(map[string]resilience.Executor, len(t.Cfg.Resilience))
	for name, p := range t.Cfg.Resilience {
		exec, err := driver.NewExecutor(resilience.Policy{
			RateLimit:      p.RateLimit,
			Burst:          p.Burst,
			ErrorThreshold: p.ErrorThreshold,
			OpenDuration:   p.OpenDuration,
			MaxConcurrent:  p.MaxConcurrent,
			MaxRetries:     p.MaxRetries,
			Timeout:        p.Timeout,
		})
		if err != nil {
			return nil, fmt.Errorf("resilience policy %q: %w", name, err)
		}
		out[name] = exec
	}
	return out, nil
}

// compileRoute assembles one Route: predicates, an upstream, the resilience
// executor its policy references, and the filter chain wrapping the proxy.
func (t *RouteTable) compileRoute(id string, raw RouteRaw, execs map[string]resilience.Executor) (*Route, error) {
	preds, err := buildPredicates(raw)
	if err != nil {
		return nil, err
	}

	up, err := parseUpstream(raw.Upstream.Target, raw.Upstream.Balancer, raw.Upstream.Discovery)
	if err != nil {
		return nil, err
	}

	var exec resilience.Executor
	if name := strings.TrimSpace(raw.Resilience.Policy); name != "" {
		e, ok := execs[name]
		if !ok {
			return nil, fmt.Errorf("unknown resilience policy %q", name)
		}
		exec = e
	}

	proxy, err := t.newProxyHandler(id, up, exec)
	if err != nil {
		return nil, err
	}

	filters, err := t.buildFilters(raw.Filters)
	if err != nil {
		return nil, err
	}

	// Wrap the proxy with the filters, outermost first, then a fixed base that
	// stamps the route id into the context and records metrics.
	h := proxy
	for i := len(filters) - 1; i >= 0; i-- {
		h = filters[i](h)
	}
	handler := t.instrument(id, h)

	return &Route{ID: id, Predicates: preds, Filters: filters, Upstream: up, handler: handler}, nil
}

// buildFilters parses a route's filter list into an ordered slice of Filters.
// Self-contained filters come from the registry; bean-backed ones (jwt-auth,
// lua) are resolved by name from the injected wrapper map.
func (t *RouteTable) buildFilters(spec string) ([]Filter, error) {
	tokens, err := splitFilters(spec)
	if err != nil {
		return nil, err
	}
	var out []Filter
	for _, tok := range tokens {
		name, args := tok.name, tok.args
		switch name {
		case "jwt-auth", "lua":
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return nil, &parseError{what: name + " requires a wrapper bean name", token: strings.Join(args, ",")}
			}
			w, ok := t.Wrappers[strings.TrimSpace(args[0])]
			if !ok {
				return nil, fmt.Errorf("gateway: no FilterWrapper bean named %q for %s filter (export it as gateway.FilterWrapper)", args[0], name)
			}
			out = append(out, w.Wrap)
		default:
			factory, ok := lookupFilter(name)
			if !ok {
				return nil, &parseError{what: "unknown filter", token: name}
			}
			f, err := factory(args)
			if err != nil {
				return nil, err
			}
			out = append(out, f)
		}
	}
	return out, nil
}

// filterToken is one parsed "name(args...)" entry from a filter list.
type filterToken struct {
	name string
	args []string
}

// splitFilters parses "a(1),b(x,y),c" into tokens, splitting on commas only at
// paren depth 0 so argument commas (addRequestHeader(X-From,gw)) are preserved.
func splitFilters(spec string) ([]filterToken, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var tokens []filterToken
	depth := 0
	start := 0
	flush := func(end int) error {
		part := strings.TrimSpace(spec[start:end])
		if part == "" {
			return nil
		}
		tok, err := parseFilterToken(part)
		if err != nil {
			return err
		}
		tokens = append(tokens, tok)
		return nil
	}
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, &parseError{what: "filter parentheses", token: spec}
			}
		case ',':
			if depth == 0 {
				if err := flush(i); err != nil {
					return nil, err
				}
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, &parseError{what: "unbalanced filter parentheses", token: spec}
	}
	if err := flush(len(spec)); err != nil {
		return nil, err
	}
	return tokens, nil
}

// parseFilterToken splits "name(a,b)" into its name and args; a bare "name"
// yields no args.
func parseFilterToken(s string) (filterToken, error) {
	open := strings.IndexByte(s, '(')
	if open < 0 {
		return filterToken{name: s}, nil
	}
	if !strings.HasSuffix(s, ")") {
		return filterToken{}, &parseError{what: "filter token", token: s}
	}
	name := strings.TrimSpace(s[:open])
	inner := s[open+1 : len(s)-1]
	var args []string
	for _, a := range strings.Split(inner, ",") {
		args = append(args, strings.TrimSpace(a))
	}
	return filterToken{name: name, args: args}, nil
}

// parseUpstream parses a route's upstream target into an Upstream. A target of
// lb://<service> is discovery-backed; http(s)://host[:port] is direct.
func parseUpstream(target, balancer, disc string) (*Upstream, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, &parseError{what: "missing upstream target", token: ""}
	}
	if svc, ok := strings.CutPrefix(target, "lb://"); ok {
		if svc == "" {
			return nil, &parseError{what: "lb:// upstream without a service name", token: target}
		}
		return &Upstream{Service: svc, Balancer: balancer, Discovery: disc}, nil
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &parseError{what: "upstream target (want lb://name or http(s)://host)", token: target}
	}
	return &Upstream{URL: u}, nil
}
