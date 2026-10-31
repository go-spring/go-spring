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

// Package discovery defines a framework-agnostic abstraction for client-side
// service discovery, with no dependency outside this repository.
//
// It answers one question for infrastructure clients (Redis, MySQL, MongoDB,
// Kafka, ...): "given a logical service name, which live host:port addresses
// can I connect to right now?". It deliberately says nothing about the
// provider-side registration of RPC frameworks — that stays bound to each
// framework (dubbo-go, kitex, ...).
//
// A company adapts its own naming service by implementing the single
// [Discovery] interface; each backend is a named bean in the IoC container
// (a discovery starter derives one per ${spring.discovery.<backend>.<name>}
// block, named "<backend>.<name>" — e.g. "etcd.main"). Every client cites the
// backend it wants by that name: the container is the discovery directory, and
// [Manager] is its object form — the one bean that hands a backend out by name.
// The runtime composition root carries that manager, so a client whose single
// injection point is the runtime reaches discovery through it like every other
// facility.
//
// This package is the READ side, and it is data plane: [Resolver] and the
// in-process endpoint snapshot it reads are consumed by loadbalance on the
// request path. Publishing this process to a registry (the provider-side write)
// is the control-plane half and lives in
// [go-spring.org/cloud/governance/controlplane/registration]; each backend starter
// contributes both adapters.
package discovery

import (
	"context"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/errutil"
)

// Reserved metadata keys. The dimensions consumers route on (version, zone,
// scheme) travel inside [Endpoint.Metadata] (and the registration side's
// Instance metadata) so backends store them without per-backend mapping. These
// keys are the single source of truth for that convention: the write side
// (registration.Server) folds the ${spring.discovery} identity fields in under
// these keys, and the read side (backends restoring [Endpoint.Scheme],
// zone-aware balancing) reads them back.
const (
	// MetaKeyVersion carries the application version of the instance.
	MetaKeyVersion = "version"

	// MetaKeyZone carries the availability zone of the instance.
	MetaKeyZone = "zone"

	// MetaKeyScheme carries the transport hint, restored to
	// [Endpoint.Scheme] by discovery backends.
	MetaKeyScheme = "scheme"
)

// Endpoint is a single connectable instance returned by a [Discovery] backend.
type Endpoint struct {
	// Addr is the connectable "host:port".
	Addr string

	// Scheme selects the transport for this instance: "" or "tcp" for a plain
	// TCP dial (the default), "tls"/"https" when it requires TLS, "http"/
	// "https" for HTTP-level routing in a gateway. It is advisory — transport
	// clients use it to pick a dialer or TLS config — and lets one service name
	// expose both plain and secure instances. Backends that do not distinguish
	// schemes leave it empty.
	Scheme string

	// Weight is the load-balancing weight. 0 set through the naming service is
	// the runtime drain signal: load-balance pools exclude the instance from
	// picking (falling back to an even split only when every instance is
	// zero-weighted, so unnormalized snapshots never blackhole). Negative
	// values are misconfiguration, treated as the default weight (1). Backends
	// that do not carry weights leave it zero — registrants normalize an
	// unset weight to 1 at write time so "default" is never stored as 0.
	Weight int

	// Disabled reports whether the instance has been administratively removed
	// from rotation — operator drain, maintenance mode, Nacos enabled=false, a
	// Kubernetes not-ready endpoint. A disabled instance must never receive
	// traffic, not even as a fallback when no healthy instance exists.
	//
	// The zero value (false) means "not disabled", so backends that do not
	// track the distinction are treated as enabled. It is deliberately
	// separate from Healthy: Disabled is an
	// operator decree that holds regardless of probe results, while Healthy is
	// a probe result a load balancer may downgrade from. The eligible set is
	// the !Disabled && Healthy instances, degrading to !Disabled only when none
	// are healthy — Disabled ones never enter either set.
	Disabled bool

	// Healthy reports whether the discovery source considers this instance
	// healthy. Backends that do not track health should leave it false; callers
	// then treat all non-disabled endpoints as eligible.
	Healthy bool

	// Metadata carries backend-specific attributes (zone, unit, version, ...),
	// passed through untouched for the caller to route on. It is owned by the
	// backend's cache and is READ-ONLY: a consumer must not write to it, since
	// the same map is shared by every reader of the snapshot.
	Metadata map[string]string
}

// Discovery is the single interface a company adapts to its naming service.
//
// It owns only naming — "given a logical name, which live addresses exist?" —
// and nothing more. Selection policy (round-robin, weighted, consistent-hash,
// zone-aware) and traffic feedback (failure counting, outlier ejection) belong
// one layer up in package loadbalance: those are per-consumer, per-request
// concerns, while a Discovery backend is shared and changes on the slow
// topology timescale. Keeping Discovery free of policy and feedback is what
// lets one naming adapter serve every client.
//
// Execution model: Resolve is a snapshot read — ctx bounds the call (deadline,
// cancellation, trace). The freshness machinery (backend watch, subscription,
// poll) lives INSIDE each backend, not in this interface: a backend keeps its
// cache current on its own, so Resolve is a cheap read from an up-to-date
// cache. A client therefore just calls Resolve whenever it needs endpoints:
//
//	// Cold start may pay a first-fetch latency; later calls read the cache.
//	rctx, rcancel := context.WithTimeout(ctx, 3*time.Second)
//	eps, err := d.Resolve(rctx, "order-service")
//	rcancel()
//
// Implementations must be safe for concurrent use.
type Discovery interface {
	// Resolve returns the current, UNNARROWED snapshot for name: every address
	// the service exposes, in every scheme, with no metadata filtering. It may
	// block on the first call for a service (seed fetch); a real backend
	// performs network I/O there, so ctx is the call's deadline/cancellation.
	//
	// The result is READ-ONLY and owned by the backend: an implementation may
	// return its live cache directly, so a caller must not assign to, reorder,
	// or append to the slice, and must not write to any [Endpoint.Metadata] map.
	// Consumers in this framework only read it. A caller that needs to keep or
	// modify the set makes its own copy (slices.Clone), paying for it explicitly
	// instead of taxing every read on the request path.
	//
	// Narrowing is deliberately not a parameter. It is the consumer's job, so
	// an adapter has exactly one thing to get right — "which addresses exist for
	// this name?" — and every caller gets the same filtering semantics no matter
	// which backend answered. [NewResolver] is where a consumer's static scheme
	// is bound; a caller that wants a bare snapshot applies the runtime
	// dimensions itself with [FilterByMetadata].
	Resolve(ctx context.Context, name string) ([]Endpoint, error)
}

// Resolver is a bound by-name read of one logical service's live endpoint
// snapshot. It pairs a [Discovery] backend with a single service name (plus a
// fixed scheme) so a consumer can ask "which endpoints for this service?"
// without re-carrying the backend and service name on every call. Endpoint
// selection — which one to use — is deliberately NOT here; it belongs in
// loadbalance, which consumes a Resolver as its endpoint source.
//
// A Resolver always surfaces errors: it is a thin, honest re-read of
// [Discovery.Resolve] (which is a cheap in-memory read of a cache-backed
// backend, since freshness lives inside the backend). Callers that want a
// selection layer feed it to loadbalance.NewPool; callers that want a bare
// snapshot call it directly.
//
// ctx is the CALLER's context — the request whose endpoint selection this is —
// and is forwarded to [Discovery.Resolve] unchanged. It must not be a
// constructor or background context: some backends do I/O on a read (a lazily
// refreshed DNS snapshot, a per-call tagged lookup), and a request-path read
// owes that I/O its deadline, cancellation and trace. Adapters whose host SDK
// offers no context (a selector callback, a static address hook) are the only
// place that should pass context.Background().
type Resolver func(ctx context.Context) ([]Endpoint, error)

// NewResolver binds the Discovery backend d to the service name, narrowed by
// scheme, and returns a Resolver that re-reads the live snapshot on every call. It
// is the single constructor every infrastructure client starter (Redis, MySQL,
// MongoDB, ...) and discovery-aware transport (httpx, the gateway) reuses: the
// starter injects the backend bean its config cites and hands it here together
// with the service name.
//
// scheme is the static narrowing of the connection: "" (no narrowing) returns
// every scheme the service exposes; "tls"/"https"/"grpc"/... keeps only
// endpoints whose [Endpoint.Scheme] matches. It is fixed here for the Resolver's
// lifetime, because narrowing a connection is a construction-time decision, not
// a per-read one. Metadata selection is deliberately absent: it is dynamic
// policy, applied per read through [FilterByMetadata].
//
// The scheme narrowing is applied HERE, after each backend read — that is the
// one place it means something, so no backend carries it and it cannot mean
// something different per adapter.
//
// It seeds with one explicit [Discovery.Resolve] — a synchronous read of the
// current state that also fails fast when the service is unknown. The seed uses
// the construction ctx; the returned Resolver does NOT capture it, it takes the
// caller's ctx on every read (see [Resolver]).
//
// Call it only when in-process discovery applies. d being non-nil and name
// being non-empty are preconditions, not states this constructor represents: a
// client asks [go-spring.org/cloud/governance.Runtime.UsesDiscovery] once and skips
// this call entirely when it says no — which is also where mesh mode is honored
// (a sidecar owns discovery+LB, so the client dials its stable address instead).
// Keeping that decision out of this constructor makes it a pure function of its
// arguments — no environment read, and no (nil, nil) third state.
func NewResolver(ctx context.Context, d Discovery, name, scheme string) (Resolver, error) {
	if _, err := d.Resolve(ctx, name); err != nil {
		return nil, errutil.Explain(err, "discovery: resolve %q", name)
	}
	return func(ctx context.Context) ([]Endpoint, error) {
		eps, err := d.Resolve(ctx, name)
		if err != nil {
			return nil, err
		}
		return filterByScheme(eps, scheme), nil
	}, nil
}

// staticBackend is the in-memory, fixed-snapshot [Discovery] behind
// [NewStaticDiscovery]. It serves the same endpoint set for every name and
// never changes.
type staticBackend struct{ eps []Endpoint }

// NewStaticDiscovery returns a [Discovery] that serves the given fixed endpoint
// set for every service name. It is the zero-dependency reference backend: a
// real adapter would talk to a naming service (Nacos, Consul, etcd, Kubernetes,
// ...) and refresh its cache as instances come and go; this one never changes.
// Use it for examples, tests, and single-instance local setups where a live
// backend is not wanted.
func NewStaticDiscovery(eps ...Endpoint) Discovery {
	return &staticBackend{eps: eps}
}

func (b *staticBackend) Resolve(_ context.Context, _ string) ([]Endpoint, error) {
	return b.eps, nil
}

// FilterByMetadata keeps the endpoints whose [Endpoint.Metadata] satisfies s and
// returns them. It is the runtime half of narrowing, the counterpart of the
// construction-time scheme: the selection layer calls it on every read, while
// the scheme dimension is already settled for the resolver's lifetime.
//
// It is a pure function over a resolved set, so the rule means the same thing
// whichever backend produced the endpoints and no adapter has to understand it.
//
// The service name is not a dimension: it selects which endpoint set to read,
// and the endpoints a backend returns for it already belong to that service.
// Nor are [Endpoint.Healthy], [Endpoint.Disabled] or [Endpoint.Weight] —
// eligibility and picking belong to loadbalance, not to the naming read. Scheme
// is not one either: [NewResolver] has already applied it.
//
// An empty s narrows nothing and returns eps unchanged; otherwise the result is
// a new slice and eps is left untouched.
func FilterByMetadata(eps []Endpoint, s matcher.Selector) []Endpoint {
	if len(s) == 0 {
		return eps
	}
	out := make([]Endpoint, 0, len(eps))
	for _, e := range eps {
		if s.Matches(e.Metadata) {
			out = append(out, e)
		}
	}
	return out
}

// filterByScheme keeps the endpoints whose [Endpoint.Scheme] matches scheme, or
// the whole slice when scheme is "". It is the construction-time half of
// narrowing and stays unexported: only [NewResolver] may apply it, so a scheme
// narrowing cannot be re-implemented differently at a call site. The runtime
// half is [FilterByMetadata].
//
// The empty scheme and "tcp" are treated as equivalent ("plain TCP"), mirroring
// the [Endpoint.Scheme] convention: scheme "tcp" therefore selects both "" and
// "tcp" endpoints. Any other value matches exactly.
func filterByScheme(eps []Endpoint, scheme string) []Endpoint {
	if scheme == "" {
		return eps
	}
	want := normalizeScheme(scheme)
	out := make([]Endpoint, 0, len(eps))
	for _, e := range eps {
		if normalizeScheme(e.Scheme) == want {
			out = append(out, e)
		}
	}
	return out
}

// normalizeScheme collapses the empty string and "tcp" to the same plain-TCP
// bucket so they match each other; every other scheme is left as-is. Most
// backends never set Scheme, so "" is the common spelling of a plain endpoint —
// without this, scheme "tcp" would silently match nothing. Keeping this
// unexported forces all scheme comparisons through [NewResolver]'s filter.
func normalizeScheme(s string) string {
	if s == "" || s == "tcp" {
		return ""
	}
	return s
}
