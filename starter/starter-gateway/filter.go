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
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go-spring.org/log"
	"go-spring.org/spring/resilience"
)

// FilterFactory builds a self-contained [Filter] from its config arguments (the
// comma-separated tokens inside the parentheses of a filter literal). It is the
// Go-idiomatic equivalent of Spring's GatewayFilterFactory: the gateway ships a
// built-in set and applications register their own via [RegisterFilter].
//
// Factories must be self-contained — they receive only their literal args, not
// container beans. Filters that wrap an injected bean (jwt-auth, lua) are not
// registered here; they are resolved by name from the gateway's injected
// wrapper map at compile time (see compile.go).
type FilterFactory func(args []string) (Filter, error)

var (
	filterMu       sync.RWMutex
	filterRegistry = map[string]FilterFactory{}
)

// RegisterFilter makes a self-contained filter available under name. It panics
// on empty name, nil factory, or duplicate registration, matching the
// driver-registry idiom used across stdlib (discovery.Register,
// resilience.RegisterDriver) so duplicate wiring fails loudly at init.
func RegisterFilter(name string, f FilterFactory) {
	if name == "" {
		panic("gateway: register filter with empty name")
	}
	if f == nil {
		panic("gateway: register nil filter factory for " + name)
	}
	filterMu.Lock()
	defer filterMu.Unlock()
	if _, ok := filterRegistry[name]; ok {
		panic("gateway: filter already registered: " + name)
	}
	filterRegistry[name] = f
}

func lookupFilter(name string) (FilterFactory, bool) {
	filterMu.RLock()
	defer filterMu.RUnlock()
	f, ok := filterRegistry[name]
	return f, ok
}

func init() {
	RegisterFilter("stripPrefix", stripPrefixFilter)
	RegisterFilter("prefixPath", prefixPathFilter)
	RegisterFilter("addRequestHeader", headerFilter(reqHeader, headerAdd))
	RegisterFilter("setRequestHeader", headerFilter(reqHeader, headerSet))
	RegisterFilter("removeRequestHeader", headerFilter(reqHeader, headerRemove))
	RegisterFilter("addResponseHeader", headerFilter(respHeader, headerAdd))
	RegisterFilter("setResponseHeader", headerFilter(respHeader, headerSet))
	RegisterFilter("removeResponseHeader", headerFilter(respHeader, headerRemove))
	RegisterFilter("rewriteHost", rewriteHostFilter)
	RegisterFilter("preserveHostHeader", preserveHostFilter)
	RegisterFilter("requestId", requestIDFilter)
	RegisterFilter("rateLimit", rateLimitFilter)
}

// stripPrefixFilter removes the first n path segments, the way an upstream that
// is unaware of the gateway's public prefix expects. stripPrefix(2) turns
// /api/orders/42 into /orders/42.
func stripPrefixFilter(args []string) (Filter, error) {
	if len(args) != 1 {
		return nil, &parseError{what: "stripPrefix args", token: strings.Join(args, ",")}
	}
	n, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || n < 0 {
		return nil, &parseError{what: "stripPrefix count", token: args[0]}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			segs := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
			if n <= len(segs) {
				r.URL.Path = "/" + strings.Join(segs[n:], "/")
			} else {
				r.URL.Path = "/"
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// prefixPathFilter prepends a fixed prefix to the request path.
func prefixPathFilter(args []string) (Filter, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return nil, &parseError{what: "prefixPath args", token: strings.Join(args, ",")}
	}
	prefix := "/" + strings.Trim(strings.TrimSpace(args[0]), "/")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = prefix + r.URL.Path
			next.ServeHTTP(w, r)
		})
	}, nil
}

type headerTarget int

const (
	reqHeader headerTarget = iota
	respHeader
)

type headerOp int

const (
	headerAdd headerOp = iota
	headerSet
	headerRemove
)

// headerFilter builds add/set/remove filters for either request or response
// headers. add/set take (name,value); remove takes (name).
func headerFilter(target headerTarget, op headerOp) FilterFactory {
	return func(args []string) (Filter, error) {
		if op == headerRemove {
			if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
				return nil, &parseError{what: "removeHeader args", token: strings.Join(args, ",")}
			}
		} else if len(args) != 2 {
			return nil, &parseError{what: "header args", token: strings.Join(args, ",")}
		}
		key := strings.TrimSpace(args[0])
		var val string
		if op != headerRemove {
			val = strings.TrimSpace(args[1])
		}
		apply := func(h http.Header) {
			switch op {
			case headerAdd:
				h.Add(key, val)
			case headerSet:
				h.Set(key, val)
			case headerRemove:
				h.Del(key)
			}
		}
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if target == reqHeader {
					apply(r.Header)
					next.ServeHTTP(w, r)
					return
				}
				apply(w.Header())
				next.ServeHTTP(w, r)
			})
		}, nil
	}
}

// rewriteHostFilter overrides the outbound Host header with a fixed value.
func rewriteHostFilter(args []string) (Filter, error) {
	if len(args) != 1 || strings.TrimSpace(args[0]) == "" {
		return nil, &parseError{what: "rewriteHost args", token: strings.Join(args, ",")}
	}
	host := strings.TrimSpace(args[0])
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Host = host
			next.ServeHTTP(w, r)
		})
	}, nil
}

// preserveHostFilter keeps the incoming Host header instead of letting the proxy
// replace it with the upstream host. It sets a request-scoped marker the proxy
// Director honors.
func preserveHostFilter(args []string) (Filter, error) {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*r = *r.WithContext(context.WithValue(r.Context(), preserveHostKey{}, true))
			next.ServeHTTP(w, r)
		})
	}, nil
}

type preserveHostKey struct{}

// requestIDFilter ensures every forwarded request carries an X-Request-Id,
// generating one when absent so it can be correlated in upstream logs.
func requestIDFilter(args []string) (Filter, error) {
	header := "X-Request-Id"
	if len(args) == 1 && strings.TrimSpace(args[0]) != "" {
		header = strings.TrimSpace(args[0])
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(header) == "" {
				r.Header.Set(header, newRequestID())
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// rateLimitFilter throttles matching requests using the resilience RateLimiter
// abstraction. Args are key=value pairs: rate (req/s, required), burst, driver
// (default "default"; a redis driver gives cross-replica limiting), algorithm
// ("token-bucket"/"sliding-window"), key ("route"/"ip"). The route id is the
// default bucket key so independent routes get independent budgets.
func rateLimitFilter(args []string) (Filter, error) {
	kv, err := parseKV(args)
	if err != nil {
		return nil, err
	}
	pol := resilience.LimitPolicy{Algorithm: resilience.Algorithm(kv["algorithm"])}
	if s := kv["rate"]; s != "" {
		if pol.Rate, err = strconv.ParseFloat(s, 64); err != nil {
			return nil, &parseError{what: "rateLimit rate", token: s}
		}
	}
	if pol.Rate <= 0 {
		return nil, &parseError{what: "rateLimit rate (must be > 0)", token: kv["rate"]}
	}
	if s := kv["burst"]; s != "" {
		if pol.Burst, err = strconv.Atoi(s); err != nil {
			return nil, &parseError{what: "rateLimit burst", token: s}
		}
	}
	driver := kv["driver"]
	if driver == "" {
		driver = "default"
	}
	d, err := resilience.MustGetLimiter(driver)
	if err != nil {
		return nil, err
	}
	limiter, err := d.NewRateLimiter(pol)
	if err != nil {
		return nil, err
	}
	keyMode := kv["key"] // "" or "route" -> per-route budget; "ip" -> per client
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := routeIDFromContext(r.Context())
			if keyMode == "ip" {
				key = key + "|" + clientIP(r)
			}
			ok, err := limiter.Allow(r.Context(), key)
			if err != nil {
				// Fail-open on limiter backend errors (e.g. redis blip): a broken
				// limiter must not take the gateway down, only log it.
				log.Warnf(r.Context(), log.TagAppDef, "gateway: rate limiter error: %v", err)
			} else if !ok {
				http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

// parseKV parses key=value filter args into a map, trimming spaces.
func parseKV(args []string) (map[string]string, error) {
	out := map[string]string{}
	for _, a := range args {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		k, v, ok := strings.Cut(a, "=")
		if !ok {
			return nil, &parseError{what: "filter key=value arg", token: a}
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

// clientIP extracts the client address for per-IP limiting, honoring a single
// X-Forwarded-For hop when present.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
