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

	"go-spring.org/log"
	"go-spring.org/spring/discovery"
	"go-spring.org/spring/loadbalance"
	"go-spring.org/spring/resilience"
)

// picker chooses the concrete target address for one request and returns a done
// callback the proxy invokes with the request outcome (feeding load-balancer
// accounting and outlier ejection). For a direct upstream the address is fixed
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
		res, err := pool.Pick(loadbalance.PickInfo{Ctx: r.Context(), HashKey: clientIP(r)})
		if err != nil {
			return nil, nil, err
		}
		u := &url.URL{Scheme: "http", Host: res.Endpoint.Addr}
		done := func(err error) {
			if res.Done != nil {
				res.Done(loadbalance.DoneInfo{Err: err})
			}
		}
		return u, done, nil
	}, nil
}

// poolFor returns the load-balancing pool for an lb:// service, building (and
// caching) the live dialer once per (discovery,service) pair. The balancer is
// per-upstream so different routes to the same service may use different
// strategies while sharing one discovery watch.
func (t *RouteTable) poolFor(up *Upstream) (*loadbalance.Pool, error) {
	disName := up.Discovery
	if disName == "" {
		disName = t.discovery
	}
	dialer, err := t.liveDialer(disName, up.Service)
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
	return loadbalance.NewPool(dialer, bal), nil
}

// liveDialer returns a cached LiveDialer for name, creating one (and its
// background watch) on first use. Dialers are keyed by discovery backend + name
// and reused across route-table recompiles.
func (t *RouteTable) liveDialer(disName, name string) (*discovery.LiveDialer, error) {
	if disName == "" {
		return nil, &parseError{what: "lb:// upstream without a discovery backend (set upstream.discovery or spring.gateway.discovery)", token: name}
	}
	key := disName + "|" + name
	t.dialerMu.Lock()
	defer t.dialerMu.Unlock()
	if d, ok := t.dialers[key]; ok {
		return d, nil
	}
	dis, err := discovery.MustGet(disName)
	if err != nil {
		return nil, err
	}
	d, err := discovery.NewLiveDialer(t.ctx, dis, name)
	if err != nil {
		return nil, err
	}
	t.dialers[key] = d
	return d, nil
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
