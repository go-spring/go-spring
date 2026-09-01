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
//   - discovery — when a ServiceName is given (and mesh mode is off), a
//     [discovery.Resolver] keeps a fresh endpoint snapshot via Watch; in mesh
//     mode a sidecar owns discovery+LB, so this layer is skipped;
//   - loadbalance — a [loadbalance.Pool] picks one live endpoint per request
//     (any of the registered strategies, plus optional outlier suspension) and the
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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/cloud/loadbalance"
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

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string

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

	// SuspendThreshold is the consecutive-failure count that suspends an endpoint
	// from the pool (outlier suspension). 0 disables suspension.
	SuspendThreshold int

	// SuspendFor is how long a suspended endpoint stays out before a half-open
	// trial. Ignored when SuspendThreshold is 0.
	SuspendFor time.Duration

	// ResilienceDriver names the registered resilience backend to protect calls
	// with. Empty disables resilience (the chain is a transparent pass-through).
	ResilienceDriver string

	// ResiliencePolicy is the backend-neutral protection applied when
	// ResilienceDriver is set.
	ResiliencePolicy resilience.Policy

	// Executor is a pre-built resilience executor to use in place of the
	// ResilienceDriver+ResiliencePolicy pair. When non-nil it takes precedence:
	// the caller builds the executor (e.g. from a centralized governance center
	// that owns its hot-reload), and httpx only wraps the transport with it. This
	// lets a caller attach a policy that refreshes externally without httpx
	// knowing about the governance source. WrapExec still wraps it (fault/observe).
	Executor resilience.Executor

	// WrapExec, if non-nil, wraps the resilience executor before it drives the
	// round-tripper — e.g. a client starter passes resilience.WrapExecutor to
	// attach span+metric+log, keeping this otel-free core free of any observe
	// import. nil-safe: leave nil for an unwrapped executor.
	WrapExec func(resilience.Executor) resilience.Executor

	// Base is the underlying transport every request ultimately flows through.
	// Starters pass an otel-instrumented transport here so trace context rides
	// along; nil means http.DefaultTransport.
	Base http.RoundTripper

	// WrapTransport, when non-nil, receives the fully assembled transport
	// (otel base → discovery/LB → resilience → traffic) and returns the
	// outermost RoundTripper to use. It is the user extension seam — the place
	// to bolt on a custom metric, an auth header, a request filter, or any
	// cross-cutting concern — without having to replace the whole chain or
	// disable the built-in layers. Applied outermost, so it sees each request
	// before the built-in layers; to detect load-test traffic from a wrapper,
	// read traffic.IsLoadTest(req.Context()) (the ctx is tagged at every layer).
	WrapTransport func(http.RoundTripper) http.RoundTripper
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

	var rsv *discovery.Resolver
	closeFns := []func() error{}

	// Discovery + load balancing. NewResolver returns (nil, nil) — "discovery
	// not in effect" — when no service name is configured or mesh mode is on
	// (a sidecar then owns discovery+LB); in that case requests flow to whatever
	// host the caller set (the service's stable mesh address).
	rsv, err = discovery.NewResolver(context.Background(), cfg.Discovery, cfg.ServiceName, discovery.WithScheme(cfg.Scheme))
	if err != nil {
		return nil, nil, err
	}
	if rsv != nil {
		closeFns = append(closeFns, rsv.Stop)

		balName := cfg.Balancer
		if balName == "" {
			balName = loadbalance.RoundRobin
		}
		bal, err := loadbalance.New(balName)
		if err != nil {
			_ = rsv.Stop()
			return nil, nil, err
		}

		var opts []loadbalance.PoolOption
		if cfg.SuspendThreshold > 0 {
			t := loadbalance.NewTracker(loadbalance.TrackerConfig{
				Threshold:  cfg.SuspendThreshold,
				SuspendFor: cfg.SuspendFor,
			})
			opts = append(opts, loadbalance.WithTracker(t))
		}
		// *discovery.Resolver satisfies loadbalance.EndpointSource via its
		// Endpoints() method, so the Pool follows the naming service in real time.
		pool := loadbalance.NewPool(rsv, bal, opts...)
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
	if cfg.Executor != nil {
		// A pre-built executor (supplied by a caller that owns its lifecycle and
		// hot-reload, e.g. a governance center) replaces the built-in
		// driver+policy path. WrapExec still applies (fault/observe wrapping).
		exec := cfg.Executor
		if cfg.WrapExec != nil {
			exec = cfg.WrapExec(exec)
		}
		base = resilience.NewRoundTripper(base, exec, nil)
		closeFns = append(closeFns, exec.Close)
	} else if cfg.ResilienceDriver != "" {
		exec, err := resilience.NewExecutor(cfg.ResilienceDriver, cfg.ResiliencePolicy)
		if err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
		if cfg.WrapExec != nil {
			exec = cfg.WrapExec(exec)
		}
		base = resilience.NewRoundTripper(base, exec, nil)
		closeFns = append(closeFns, exec.Close)
	}

	// Traffic: inject the load-test marker onto every outbound request when the
	// call's ctx carries it, so downstream hops can recognise synthetic load and
	// route/isolate/tag it. Sits above resilience so each retry attempt carries
	// the marker (the header is set on the original request, which the retry
	// loop reuses), and below the user WrapTransport so a custom wrapper can
	// still observe or override the header. Inert — and effectively free —
	// unless traffic.IsLoadTest(ctx) is true.
	base = &trafficTransport{base: base}

	// User extension seam: the outermost layer, applied last so it wraps the
	// complete built-in stack (traffic included).
	if cfg.WrapTransport != nil {
		base = cfg.WrapTransport(base)
	}

	return base, func() error { return closeAll(closeFns) }, nil
}

// trafficTransport injects the load-test marker header onto each request when
// the request's context is a load-test context, then delegates to base. It is
// the outbound seam for [go-spring.org/cloud/governance/traffic] on the HTTP client path.
type trafficTransport struct {
	base http.RoundTripper
}

func (t *trafficTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	traffic.InjectHTTP(req.Context(), req)
	return t.base.RoundTrip(req)
}

// balancedTransport rewrites each request to a live endpoint chosen by the pool
// and reports the outcome back so least-conn accounting and outlier suspension see
// every call. It sits below the resilience layer, so retries pick afresh.
type balancedTransport struct {
	base http.RoundTripper
	pool *loadbalance.Pool
}

func (t *balancedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ep, err := t.pool.Pick(loadbalance.PickInfo{})
	if err != nil {
		return nil, err
	}

	// Clone before mutating: net/http may retry and the resilience layer above
	// reuses the original request across attempts.
	r := req.Clone(req.Context())
	r.URL.Host = ep.Addr
	r.Host = ep.Addr

	resp, err := t.base.RoundTrip(r)
	t.pool.Complete(ep, err)
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
