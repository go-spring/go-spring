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
//     [discovery.NewResolver] keeps a fresh endpoint snapshot (freshness lives
//     inside the backend); in mesh
//     mode a sidecar owns discovery+LB, so this layer is skipped;
//   - loadbalance — a [loadbalance.Pool] picks one live endpoint per request
//     (any of the registered strategies, plus optional outlier suspension) and the
//     transport rewrites the request host to it;
//   - resilience — an executor wraps the whole chain so rate limiting, circuit
//     breaking and retry protect every call; because it sits outside the
//     balancer, a retry re-picks a fresh endpoint and the breaker keys on the
//     logical service name. When no explicit executor is supplied, one is
//     resolved from the centralized governance authority under Resource;
//   - TLS — the certificate surface for https targets is built into the base
//     transport;
//
// Observability is built in, not layered on by a starter: the base transport is
// wrapped with otelhttp so every call carries a client span, and an active
// resilience executor is wrapped with fault + observe so each call emits
// outcome-classified metrics and an access log (gated by Observability). With no
// OTel SDK registered these are no-ops, so the transport stays usable bare. The
// package never imports a concrete starter.
package httpx

import (
	"context"
	"net/http"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/tlsconf"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
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

	// TLS configures the certificate surface for https targets (client key pair,
	// CA bundle, expected peer name, insecure escape hatch). Off by default; when
	// enabled it is wired into the base transport, so Scheme "https"/"tls" gets
	// verifiable TLS instead of system defaults. Ignored when Base is set — an
	// explicit Base owns the dialer.
	TLS tlsconf.TLSConfig

	// Resource is the governance resource label protecting this client (e.g.
	// "http:user-svc"). When empty it is derived as
	// resilience.ResourceLabel("http", ServiceName, Addr) — service-name wins
	// whenever set, only an entry with no service-name falls back to its
	// address, so the label stays stable across addressing-mode switches.
	// Set it explicitly to decouple the label from addressing entirely.
	Resource string

	// ResilienceDriver names the registered resilience backend to protect calls
	// with. When neither Executor nor ResilienceDriver is set, the executor is
	// resolved from the centralized governance authority (governance.Register
	// under Resource); with governance off the armed policy is zero and the
	// executor is a transparent pass-through.
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

	// WrapExec, when non-nil, replaces the default executor wrap (which layers
	// observe over fault over the raw executor) — the escape hatch for a caller
	// that needs its own ordering or extra layers. nil means the default wrap.
	WrapExec func(resilience.Executor) resilience.Executor

	// Base is the raw underlying transport every request ultimately flows
	// through — tracing is layered on top of it by this package, so pass the
	// plain dialer (a TLS-configured clone), not an instrumented one. nil means
	// http.DefaultTransport.
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
	// Base dialer: an explicit Base wins; otherwise DefaultTransport, cloned with
	// the configured TLS surface when enabled (nil BuildClient keeps system
	// defaults, so a plain http route is untouched).
	base := cfg.Base
	if base == nil {
		base = http.DefaultTransport
		if tlsCfg, e := cfg.TLS.BuildClient(); e != nil {
			return nil, nil, e
		} else if tlsCfg != nil {
			t := http.DefaultTransport.(*http.Transport).Clone()
			t.TLSClientConfig = tlsCfg
			base = t
		}
	}
	// Trace: wrap the raw base with otelhttp so every call carries a client
	// span and propagates trace context. A no-op (and effectively free) when no
	// OTel SDK is registered.
	base = otelhttp.NewTransport(base)

	var resolver discovery.Resolver
	closeFns := []func() error{}

	// Discovery + load balancing. NewResolver returns (nil, nil) — "discovery
	// not in effect" — when no service name is configured or mesh mode is on
	// (a sidecar then owns discovery+LB); in that case requests flow to whatever
	// host the caller set (the service's stable mesh address). Freshness lives
	// inside the discovery backend, so the resolver has no resources to release.
	resolver, err = discovery.NewResolver(context.Background(), cfg.Discovery, cfg.ServiceName, discovery.WithScheme(cfg.Scheme))
	if err != nil {
		return nil, nil, err
	}
	if resolver != nil {
		balName := cfg.Balancer
		if balName == "" {
			balName = loadbalance.RoundRobin
		}
		bal, err := loadbalance.New(balName)
		if err != nil {
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
		// The resolver (bound by-name re-read of the backend snapshot) feeds the
		// Pool as its endpoint source, so it follows the naming service in real
		// time.
		pool := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal, opts...)
		base = &balancedTransport{base: base, pool: pool}
	} else if cfg.Addr != "" {
		// Direct mode: pin every request to the configured address so callers
		// need not set a Target on the generated client.
		base = &fixedHostTransport{base: base, addr: cfg.Addr}
	}

	// Resilience wraps the (possibly balanced) transport so a retry re-enters
	// the balancer and picks a fresh endpoint, and the breaker keys on the host
	// carried by the generated client (the logical service name in discovery
	// mode). Three executor sources, first match wins: a pre-built Executor, an
	// explicit ResilienceDriver+Policy, or the centralized governance authority
	// under Resource (the default — with governance off the armed policy is
	// zero and the executor is a transparent pass-through, with hot-reload
	// through the governance subscription when it is on).
	var exec resilience.Executor
	switch {
	case cfg.Executor != nil:
		exec = cfg.Executor
	case cfg.ResilienceDriver != "":
		if exec, err = resilience.NewExecutor(cfg.ResilienceDriver, cfg.ResiliencePolicy); err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
	default:
		if exec, err = governedExecutor(cfg.resource()); err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
	}

	// Default wrap: observe( fault( rawExec ) ) — fault injects innermost so a
	// fault looks like a real downstream failure, observe records the final
	// outcome (span, outcome-classified metrics, access log). WrapExec, when
	// set, replaces this stack entirely.
	wrap := cfg.WrapExec
	if wrap == nil {
		wrap = func(e resilience.Executor) resilience.Executor {
			return resilience.WrapExecutor(fault.WrapExecutor(e), "http")
		}
	}
	exec = wrap(exec)
	base = resilience.NewRoundTripper(base, exec, nil)
	closeFns = append(closeFns, exec.Close)

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
	canonical.InjectHTTP(req.Context(), req)
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

// resource derives the governance resource label: explicit Resource wins;
// otherwise service-name (whenever set) before the direct address, so the
// label a govern rule matches on stays stable across addressing-mode switches.
func (c Config) resource() string {
	if c.Resource != "" {
		return c.Resource
	}
	return resilience.ResourceLabel("http", c.ServiceName, c.Addr)
}

// minRequestsFloor is the minimum sample size enforced on an error-rate breaker
// resolved for an http resource. resilience's own zero-value floor is 1 — a
// single failure at a 100% rate trips the breaker immediately, which is too
// hair-trigger for typical http traffic. Unless a govern rule sets a HIGHER
// value explicitly, MinRequests is raised to this floor. Applies only to
// policies resolved through the governance path here; resilience core defaults
// are untouched, and the consecutive strategy is unaffected (MinRequests is an
// error-rate-only knob).
const minRequestsFloor = 5

// governedExecutor builds the resilience executor for one http resource from
// the centralized governance authority, applying the minRequestsFloor to the
// resolved policy. It subscribes through the governance facade rather than the
// resilience.ExecutorFor seam, because the floor must see the resolved
// policy — a seam executor is opaque to it. With governance off the armed
// policy is zero and the executor is a transparent pass-through, exactly as
// with ExecutorFor; hot-reload works the same way (governance re-invokes the
// subscriber, the executor Refreshes).
func governedExecutor(resource string) (resilience.Executor, error) {
	var exec resilience.Executor
	refresh := func(p resilience.Policy) {
		if e := exec; e != nil {
			_ = e.Refresh(floorMinRequests(p))
		}
	}
	p := floorMinRequests(governance.Register(resource, refresh))
	e, err := resilience.NewExecutor(governance.Driver(), p)
	if err != nil {
		return nil, err
	}
	exec = e
	return exec, nil
}

// floorMinRequests raises an error-rate policy's MinRequests to
// minRequestsFloor when unset or lower. A policy that is zero or consecutive
// is returned unchanged.
func floorMinRequests(p resilience.Policy) resilience.Policy {
	if p.ResolvedBreakerStrategy() == resilience.BreakerErrorRate && p.ErrorRateThreshold > 0 && p.MinRequests < minRequestsFloor {
		p.MinRequests = minRequestsFloor
	}
	return p
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
