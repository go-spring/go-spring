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
//     logical service name. When no explicit executor is supplied, it comes from
//     the injected [resilience.Manager] under Service — the same
//     [resilience.Manager.ClientExecutorFor] path every other client starter uses;
//   - TLS — the certificate surface for https targets is built into the base
//     transport;
//
// Observability is built in, not layered on by a starter: the base transport is
// wrapped with otelhttp so every call carries a client span, and the resilience
// executor carries observe (applied by the manager when it builds the executor)
// plus fault, so each call emits outcome-classified metrics and an access log
// (gated by Observability). With no
// OTel SDK registered these are no-ops, so the transport stays usable bare. The
// package never imports a concrete starter; the governance authority reaches it
// as the beans [NewTransport] takes — a [resilience.Manager], a [fault.Injector]
// and a [loadbalance.Manager] for the selection half — never through a
// package-level seam.
package httpx

import (
	"context"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/governance"
	"net/http"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/security"
	"go-spring.org/cloud/traffic"
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

	// Discovery is the discovery backend that resolves ServiceName. The caller
	// (a starter) injects the backend bean its config cites; required when
	// ServiceName is set.
	Discovery discovery.Discovery

	// TLS configures the certificate surface for https targets (client key pair,
	// CA bundle, expected peer name, insecure escape hatch). Off by default; when
	// enabled it is wired into the base transport, so Scheme "https"/"tls" gets
	// verifiable TLS instead of system defaults. Ignored when Base is set — an
	// explicit Base owns the dialer.
	TLS security.TLSConfig

	// Service is the governance service label protecting this client (e.g.
	// "http:user-svc") — the name a governance rule matches. When empty it is derived
	// as resilience.ServiceLabel("http", ServiceName, Addr): ServiceName wins
	// whenever set, only an entry with no service-name falls back to its address,
	// so the label stays stable across addressing-mode switches. Set it
	// explicitly to decouple the label from addressing entirely. Distinct from
	// ServiceName, which is what discovery resolves; this is what governance
	// protects.
	Service string

	// ResilienceDriver is the resilience backend to protect calls with. The
	// caller resolves it by name from the container's driver directory (the
	// starter's ${...driver} entry key) and passes the driver itself, since this
	// package is container-free. When neither chain.Executor nor ResilienceDriver is
	// set, the executor is resolved from the injected [resilience.Manager] under
	// Service; with governance off the armed policy is zero and the executor is
	// a transparent pass-through.
	ResilienceDriver resilience.Driver

	// ResiliencePolicy is the backend-neutral protection applied when
	// ResilienceDriver is set.
	ResiliencePolicy resilience.ClientPolicy

	// Executor is a pre-built resilience executor to use in place of the
	// ResilienceDriver+ResiliencePolicy pair. When non-nil it takes precedence:
	// the caller builds the executor (e.g. from a centralized governance center
	// that owns its hot-reload), and httpx only wraps the transport with it. This
	// lets a caller attach a policy that refreshes externally without httpx
	// knowing about the governance source. WrapExec still wraps it (fault/observe).
	Executor chain.Executor

	// WrapExec, when non-nil, replaces the default executor wrap (fault over the
	// raw executor, with observe added here only when the executor came from the
	// driver/chain.Executor escape hatches rather than from the manager) — the escape
	// hatch for a caller that needs its own ordering or extra layers. nil means
	// the default wrap.
	WrapExec func(chain.Executor) chain.Executor

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
	// ask the injected propagator's IsLoadTest(req.Context()) (the ctx is tagged
	// at every layer).
	WrapTransport func(http.RoundTripper) http.RoundTripper
}

// NewTransport assembles the http.RoundTripper for cfg and returns it together
// with a close function that releases what the transport owns — an executor it
// built itself, the discovery watch. An executor taken from [resilience.Manager]
// is not released here: the manager owns it for the process lifetime. It fails
// fast when ServiceName is set but the discovery backend or load-balancing
// strategy cannot be resolved, so misconfiguration surfaces at wiring time
// rather than on the first request.
//
// center is the governance center bean the caller received from the container;
// a standalone caller passes nil, which is normalized below to fresh unarmed
// authorities.
// prop is the application's load-test convention bean (nil means go-spring's
// default), which stamps the marker onto every outbound request.
func NewTransport(cfg Config, center *governance.Center, prop traffic.Propagator) (rt http.RoundTripper, close func() error, err error) {
	// A standalone caller that has no container passes a nil center; normalize it
	// to fresh unarmed authorities so every later use is nil-free. A nil
	// resilience manager would panic on its first method call, so normalizing at
	// the assembly point keeps every other caller free of nil branches; a nil
	// fault injector is nil-safe and simply leaves the fault layer transparent.
	if center == nil {
		lb, e := loadbalance.NewManager(nil) // no factory bean contributed
		if e != nil {
			return nil, nil, e
		}
		center = governance.NewCenter(governance.Config{},
			resilience.NewManager(nil), lb,
			fault.NewInjector(fault.Configs{}, nil), discovery.NewManager(nil), nil)
	}
	if prop == nil {
		if prop, err = traffic.NewDefaultPropagator(traffic.DefaultBinding()); err != nil {
			return nil, nil, err
		}
	}

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

	// pool is the load-balancing pool when discovery is in effect, else nil. It
	// is kept outside the branch below because the governance subscription
	// further down drives it (balancer + outlier suspension).
	var pool *loadbalance.Pool

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
		// Round-robin is only the starting strategy: the governance subscription
		// below replaces it when a rule for this service names one.
		bal := loadbalance.NewRoundRobin()

		// The tracker is attached unconditionally and starts disabled
		// (Threshold 0): outlier suspension is a governance decision now, so
		// the policy resolved below turns it on — and can turn it off again —
		// without rebuilding the transport. Attaching a disabled tracker has no
		// effect on routing, so this costs nothing when governance is off.
		//
		// The resolver (bound by-name re-read of the backend snapshot) feeds the
		// Pool as its endpoint source, so it follows the naming service in real
		// time.
		pool = loadbalance.NewPool(resolver, bal)
		base = &balancedTransport{base: base, pool: pool}
	} else if cfg.Addr != "" {
		// Direct mode: pin every request to the configured address so callers
		// need not set a Target on the generated client.
		base = &fixedHostTransport{base: base, addr: cfg.Addr}
	}

	// Resilience wraps the (possibly balanced) transport so a retry re-enters
	// the balancer and picks a fresh endpoint, and the breaker keys on
	// cfg.service() — the same label the policy is resolved under, so limiter/
	// breaker state and driver rule names agree with the governance rule that armed
	// them. Three executor sources, first match wins: a pre-built chain.Executor, an
	// explicit ResilienceDriver+Policy, or — the default, and the same path every
	// other client starter takes — the injected [resilience.Manager] under
	// Service. The manager owns that executor: it is built once per label, shared
	// by every user of the label, refreshes itself on a policy change, and
	// already carries the observe layer; with governance off the armed policy is
	// zero and it is a transparent pass-through.
	var exec chain.Executor
	managerOwned := false
	switch {
	case cfg.Executor != nil:
		exec = cfg.Executor
	case cfg.ResilienceDriver != nil:
		if exec, err = cfg.ResilienceDriver.NewClientExecutor(cfg.service(), cfg.ResiliencePolicy); err != nil {
			closeAll(closeFns)
			return nil, nil, err
		}
	default:
		exec = center.Resilience().ClientExecutorFor("http", cfg.service())
		managerOwned = true
		// The pool's endpoint-selection half follows the governance rule through
		// its own subscription on the loadbalance manager — the protection policy
		// reaches the executor, the selection reaches the pool, and neither change
		// rebuilds anything. A nil pool (direct addressing) opts out.
		if pool != nil {
			center.Loadbalance().Bind(pool, cfg.service())
		}
	}

	// Default wrap: fault( observe( rawExec ) ) — observe is applied first, on
	// the still-private raw executor, so it attaches its breaker listener at
	// construction; fault then injects outside it, but the injected error still
	// flows through the real executor's retry loop and breaker. observe records
	// the final outcome (span, outcome-classified metrics, access log).
	//
	// The manager-owned executor is already instrumented — the manager applies
	// the same observe layer when it builds the backing executor, which is how
	// every client starter gets it — so only fault is added on top; an executor
	// httpx built itself gets observe here. WrapExec, when set, replaces this
	// stack entirely and receives the raw executor either way.
	wrap := cfg.WrapExec
	if wrap == nil {
		if managerOwned {
			wrap = func(e chain.Executor) chain.Executor {
				return fault.WrapClientExecutor(e, cfg.service(), center.Fault())
			}
		} else {
			wrap = func(e chain.Executor) chain.Executor {
				return fault.WrapClientExecutor(observability.WrapClientExecutor(e, "http", cfg.service()), cfg.service(), center.Fault())
			}
		}
	}
	exec = wrap(exec)
	base = resilience.NewRoundTripper(base, exec)
	// The manager owns its executor for the process lifetime, so closing it with
	// one transport would be wrong; only an httpx-built one is released here.
	if !managerOwned {
		closeFns = append(closeFns, exec.Close)
	}

	// Traffic: inject the load-test marker onto every outbound request when the
	// call's ctx carries it, so downstream hops can recognise synthetic load and
	// route/isolate/tag it. Sits above resilience so each retry attempt carries
	// the marker (the header is set on the original request, which the retry
	// loop reuses), and below the user WrapTransport so a custom wrapper can
	// still observe or override the header. Inert — and effectively free —
	// unless the propagator reports the ctx as load-test traffic.
	base = &trafficTransport{base: base, prop: prop}

	// User extension seam: the outermost layer, applied last so it wraps the
	// complete built-in stack (traffic included).
	if cfg.WrapTransport != nil {
		base = cfg.WrapTransport(base)
	}

	return base, func() error { return closeAll(closeFns) }, nil
}

// trafficTransport injects the load-test marker header onto each request when
// the request's context is a load-test context, then delegates to base. It is
// the outbound seam for [go-spring.org/cloud/traffic] on the HTTP client path.
type trafficTransport struct {
	base http.RoundTripper
	prop traffic.Propagator
}

func (t *trafficTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// The header adapter writes through Header.Set, so the key lands in
	// canonical spelling and reads back through Get.
	t.prop.Inject(req.Context(), propagate.Header(req.Header))
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

// service derives the governance service label: explicit Service wins;
// otherwise service-name (whenever set) before the direct address, so the
// label a governance rule matches on stays stable across addressing-mode switches.
func (c Config) service() string {
	if c.Service != "" {
		return c.Service
	}
	return resilience.ServiceLabel("http", c.ServiceName, c.Addr)
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
