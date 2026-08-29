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
	"net/http"
	"net/http/httputil"
	"net/url"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
)

// picker chooses the concrete target address for one request and returns a done
// callback the proxy invokes with the request outcome (feeding load-balancer
// accounting and outlier suspension). For a direct upstream the address is fixed
// and done is a no-op.
type picker func(r *http.Request) (target *url.URL, done func(err error), err error)

// buildPicker returns a picker for an upstream. A direct http(s):// upstream
// yields a fixed target; an lb://<service> upstream resolves through discovery
// and a load-balancing pool that stays fresh via a background watch. The pool is
// cached on the table so recompiles reuse the live dialer instead of leaking a
// new watch goroutine each time.
func (t *RouteTable) buildPicker(up *Upstream) (picker, error) {
	if up.URL != nil {
		u := up.URL
		return func(*http.Request) (*url.URL, func(error), error) {
			return u, func(error) {}, nil
		}, nil
	}

	pool, err := t.poolFor(up)
	if err != nil {
		return nil, err
	}
	return func(r *http.Request) (*url.URL, func(error), error) {
		ep, err := pool.Pick(loadbalance.PickInfo{HashKey: clientIP(r)})
		if err != nil {
			return nil, nil, err
		}
		u := &url.URL{Scheme: "http", Host: ep.Addr}
		done := func(err error) { pool.Complete(ep, err) }
		return u, done, nil
	}, nil
}

// poolFor returns the load-balancing pool for an lb:// service, building (and
// caching) the live dialer once per (discovery,service) pair. The balancer is
// per-upstream so different routes to the same service may use different
// strategies while sharing one discovery watch. Outlier suspension (upstream
// suspend-threshold/suspend-for) is likewise per-upstream: an instance that
// fails repeatedly is dropped from the pool's candidate set for the cool-down,
// so a zombie upstream stops generating 502s until it proves itself again.
func (t *RouteTable) poolFor(up *Upstream) (*loadbalance.Pool, error) {
	disName := up.Discovery
	if disName == "" {
		disName = t.discovery
	}
	dialer, err := t.resolver(disName, up.Service)
	if err != nil {
		return nil, err
	}
	balName := up.Balancer
	if balName == "" {
		balName = loadbalance.RoundRobin
	}
	bal, err := loadbalance.New(balName)
	if err != nil {
		return nil, err
	}
	var opts []loadbalance.PoolOption
	if up.SuspendThreshold > 0 {
		tr := loadbalance.NewTracker(loadbalance.TrackerConfig{
			Threshold:  up.SuspendThreshold,
			SuspendFor: up.SuspendFor, // 0 keeps the tracker's 5s default
		})
		opts = append(opts, loadbalance.WithTracker(tr))
	}
	return loadbalance.NewPool(dialer, bal, opts...), nil
}

// resolver returns a cached [discovery.Resolver] for name, creating one (and
// its background watch) on first use. Resolvers are keyed by discovery backend
// + name and reused across route-table recompiles.
func (t *RouteTable) resolver(disName, name string) (*discovery.Resolver, error) {
	if disName == "" {
		return nil, &parseError{what: "lb:// upstream without a discovery backend (set upstream.discovery or spring.gateway.discovery)", token: name}
	}
	key := disName + "|" + name
	t.dialerMu.Lock()
	defer t.dialerMu.Unlock()
	if d, ok := t.dialers[key]; ok {
		return d, nil
	}
	d, err := discovery.NewResolver(t.ctx, disName, name)
	if err != nil {
		return nil, err
	}
	if d == nil {
		// NewResolver yields (nil, nil) when discovery is not in effect — empty
		// name or mesh mode active. A gateway lb:// upstream needs a live
		// resolver, so surface that as a route-table compile error rather than
		// handing a nil dialer to the load-balancing pool.
		return nil, &parseError{what: "lb:// upstream cannot resolve (empty service name or mesh mode active — in mesh mode route to the service's stable address instead)", token: name}
	}
	t.dialers[key] = d
	return d, nil
}

// stopOrphanedDialers stops and removes the cached discovery resolvers whose
// (discovery,service) key is no longer referenced by any route. A service dropped
// from the route config must not keep its background watch running after a hot
// reload, so recompile calls this with the set of keys the new routes use.
func (t *RouteTable) stopOrphanedDialers(keep map[string]bool) {
	t.dialerMu.Lock()
	defer t.dialerMu.Unlock()
	for key, d := range t.dialers {
		if !keep[key] {
			_ = d.Stop()
			delete(t.dialers, key)
		}
	}
}

// Close stops the background discovery watch behind every cached resolver. It is
// the gs destroy method, invoked on graceful shutdown, so the discovery watches
// do not leak past the gateway's lifetime.
func (t *RouteTable) Destroy() error {
	t.dialerMu.Lock()
	defer t.dialerMu.Unlock()
	for key, d := range t.dialers {
		_ = d.Stop()
		delete(t.dialers, key)
	}
	return nil
}

// newProxyHandler assembles the terminal handler of a route's chain: a reverse
// proxy whose Transport is wrapped with the route's resilience executor so
// retries and circuit breaking happen on the forwarding hop. exec may be nil
// (no resilience configured), in which case NewRoundTripper returns the base
// transport unchanged.
//
// The target is picked once in the outer handler (not the Director) so a pick
// failure — no live upstream instance — is a clean 503 instead of a dial to an
// empty host. The Director then just applies the chosen target; on a resilience
// retry the same target is reused.
func (t *RouteTable) newProxyHandler(routeID string, up *Upstream, exec resilience.Executor) (http.Handler, error) {
	pick, err := t.buildPicker(up)
	if err != nil {
		return nil, err
	}

	base := http.DefaultTransport.(*http.Transport).Clone()
	transport := resilience.NewRoundTripper(base, exec, func(*http.Request) string { return routeID })

	rp := &httputil.ReverseProxy{
		Transport: transport,
		Director: func(req *http.Request) {
			tg, _ := req.Context().Value(targetKey{}).(*url.URL)
			req.URL.Scheme = tg.Scheme
			req.URL.Host = tg.Host
			if preserve, _ := req.Context().Value(preserveHostKey{}).(bool); !preserve {
				req.Host = tg.Host
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Warnf(r.Context(), log.TagAppDef, "gateway: route %q upstream error: %v", routeID, err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target, done, err := pick(r)
		if err != nil {
			log.Warnf(r.Context(), log.TagAppDef, "gateway: route %q no upstream: %v", routeID, err)
			http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		sw := &statusWriter{ResponseWriter: w}
		r = r.WithContext(context.WithValue(r.Context(), targetKey{}, target))
		rp.ServeHTTP(sw, r)
		if sw.status >= 500 {
			done(errUpstreamStatus)
		} else {
			done(nil)
		}
	}), nil
}

type targetKey struct{}

// errUpstreamStatus marks a 5xx forwarded response as a failure for the
// load-balancer's outlier tracker.
var errUpstreamStatus = &parseError{what: "upstream status", token: "5xx"}

// statusWriter captures the response status so the proxy can report success or
// failure to the load-balancer done callback.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
