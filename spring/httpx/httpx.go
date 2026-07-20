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

// Package httpx is the runtime assembler behind the declarative HTTP client
// (the OpenFeign / @HttpExchange equivalent). Go has no runtime proxy, so the
// call sites are produced by gs-http-gen; this package supplies the transport
// they run on. A generated client only holds an *http.Client, and [NewTransport]
// builds that client's http.RoundTripper by wiring together the three stdlib
// abstractions a microservice call needs, all behind the single http.RoundTripper
// seam already used by resilience and the otelhttp transport:
//
//   - discovery — when a ServiceName is given, a [discovery.LiveDialer] keeps a
//     fresh endpoint snapshot via Watch;
//   - loadbalance — a [loadbalance.Pool] picks one live endpoint per request
//     (any of the registered strategies, plus optional outlier ejection) and the
//     transport rewrites the request host to it;
//   - resilience — an optional [resilience] executor wraps the whole chain so
//     rate limiting, circuit breaking and retry protect every call; because it
//     sits outside the balancer, a retry re-picks a fresh endpoint and the
//     breaker keys on the logical service name.
//
// The package depends only on stdlib, so the generated code and this assembler
// never import a concrete starter. Trace propagation (otel) is layered on top by
// passing an instrumented Base transport, keeping observability a starter concern.
package httpx

import (
	"context"
	"net/http"
	"time"

	"go-spring.org/spring/discovery"
	"go-spring.org/spring/loadbalance"
	"go-spring.org/spring/resilience"
)

// Config describes how to assemble the transport for one declarative client.
// Exactly one addressing mode applies: leave ServiceName empty for a direct
// address (the generated client's Target is dialed as-is) or set it to route
// through service discovery and load balancing.
type Config struct {
	// ServiceName is the logical name resolved through discovery. When empty the
	// transport falls back to Addr (or, if that is empty too, the request host
	// set by the generated client) — no discovery, no load balancing.
	ServiceName string

	// Addr is the direct "host:port" used when ServiceName is empty. The transport
	// rewrites every request to it, so the injected client fully owns addressing
	// and the generated client's Target field need not be set.
	Addr string

	// Discovery names the registered discovery backend to resolve ServiceName
	// through. Required when ServiceName is set.
	Discovery string

	// Balancer names the registered load-balancing strategy (round_robin,
	// least_conn, consistent_hash, weighted, zone_aware). Defaults to round_robin.
	Balancer string

	// EjectThreshold is the consecutive-failure count that ejects an endpoint
	// from the pool (outlier ejection). 0 disables ejection.
	EjectThreshold int

	// EjectFor is how long an ejected endpoint stays out before a half-open
	// trial. Ignored when EjectThreshold is 0.
	EjectFor time.Duration

	// ResilienceDriver names the registered resilience backend to protect calls
	// with. Empty disables resilience (the chain is a transparent pass-through).
	ResilienceDriver string

	// ResiliencePolicy is the backend-neutral protection applied when
	// ResilienceDriver is set.
	ResiliencePolicy resilience.Policy

	// Base is the underlying transport every request ultimately flows through.
	// Starters pass an otel-instrumented transport here so trace context rides
	// along; nil means http.DefaultTransport.
	Base http.RoundTripper
}

// NewTransport assembles the http.RoundTripper for cfg and returns it together
// with a close function that releases the discovery watch and resilience
// executor. It fails fast when ServiceName is set but the discovery backend or
// load-balancing strategy cannot be resolved, so misconfiguration surfaces at
// wiring time rather than on the first request.
func NewTransport(cfg Config) (rt http.RoundTripper, close func() error, err error) {
	base := cfg.Base
	if base == nil {
		base = http.DefaultTransport
	}

	var ld *discovery.LiveDialer
	closeFns := []func() error{}

	// Discovery + load balancing: only when a service name is configured.
	if cfg.ServiceName != "" {
		d, err := discovery.MustGet(cfg.Discovery)
		if err != nil {
			return nil, nil, err
		}
		ld, err = discovery.NewLiveDialer(context.Background(), d, cfg.ServiceName)
		if err != nil {
			return nil, nil, err
		}
		closeFns = append(closeFns, ld.Stop)

		balName := cfg.Balancer
		if balName == "" {
			balName = loadbalance.RoundRobin
		}
		bal, err := loadbalance.New(balName)
		if err != nil {
			_ = ld.Stop()
			return nil, nil, err
		}

		var opts []loadbalance.PoolOption
		if cfg.EjectThreshold > 0 {
			t := loadbalance.NewTracker(loadbalance.TrackerConfig{
				Threshold: cfg.EjectThreshold,
				EjectFor:  cfg.EjectFor,
			})
			opts = append(opts, loadbalance.WithTracker(t))
		}
		pool := loadbalance.NewPool(ld, bal, opts...)
		base = &balancedTransport{base: base, pool: pool}
	} else if cfg.Addr != "" {
		// Direct mode: pin every request to the configured address so callers
		// need not set a Target on the generated client.
		base = &fixedHostTransport{base: base, addr: cfg.Addr}
	}

	// Resilience: wraps the (possibly balanced) transport so a retry re-enters
	// the balancer and picks a fresh endpoint, and the breaker keys on the host
	// carried by the generated client (the logical service name in discovery
	// mode). Disabled when no driver is configured, leaving base unchanged.
	if cfg.ResilienceDriver != "" {
		drv, err := resilience.MustGetDriver(cfg.ResilienceDriver)
		if err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
		exec, err := drv.NewExecutor(cfg.ResiliencePolicy)
		if err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
		base = resilience.NewRoundTripper(base, exec, nil)
		closeFns = append(closeFns, exec.Close)
	}

	return base, func() error { return closeAll(closeFns) }, nil
}

// balancedTransport rewrites each request to a live endpoint chosen by the pool
// and reports the outcome back so least-conn accounting and outlier ejection see
// every call. It sits below the resilience layer, so retries pick afresh.
type balancedTransport struct {
	base http.RoundTripper
	pool *loadbalance.Pool
}

func (t *balancedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	res, err := t.pool.Pick(loadbalance.PickInfo{Ctx: req.Context()})
	if err != nil {
		return nil, err
	}

	// Clone before mutating: net/http may retry and the resilience layer above
	// reuses the original request across attempts.
	r := req.Clone(req.Context())
	r.URL.Host = res.Endpoint.Addr
	r.Host = res.Endpoint.Addr

	resp, err := t.base.RoundTrip(r)
	if res.Done != nil {
		res.Done(loadbalance.DoneInfo{Err: err})
	}
	return resp, err
}

// fixedHostTransport pins every request to a single address (direct mode). It
// sits in the same spot as balancedTransport so the resilience layer above and
// the generated call sites below behave identically in both addressing modes.
type fixedHostTransport struct {
	base http.RoundTripper
	addr string
}

func (t *fixedHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.URL.Host = t.addr
	r.Host = t.addr
	return t.base.RoundTrip(r)
}

func closeAll(fns []func() error) error {
	var firstErr error
	for i := len(fns) - 1; i >= 0; i-- {
		if err := fns[i](); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
