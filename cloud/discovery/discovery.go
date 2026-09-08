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

// Package discovery defines a framework-agnostic, zero-dependency abstraction
// for client-side service discovery.
//
// It answers one question for infrastructure clients (Redis, MySQL, MongoDB,
// Kafka, ...): "given a logical service name, which live host:port addresses
// can I connect to right now?". It deliberately says nothing about the
// provider-side registration of RPC frameworks — that stays bound to each
// framework (dubbo-go, kitex, ...).
//
// A company adapts its own naming service by implementing the single
// [Discovery] interface; each backend is a named bean in the IoC container
// (named after its config label, e.g. ${spring.discovery.etcd.<name>}), and
// every client starter injects the backend it cites by name — the container is
// the discovery directory. Publishing this process to a registry (the
// provider-side write) is handled by a registry starter such as
// starter-registry-etcd, not by this package.
package discovery

import (
	"context"
	"fmt"

	"go-spring.org/cloud/mesh"
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
	// that do not carry weights leave it empty — registrants normalize an
	// unset weight to 1 at write time so "default" is never stored as 0.
	Weight int

	// Disabled reports whether the instance has been administratively removed
	// from rotation — operator drain, maintenance mode, Nacos enabled=false, a
	// Kubernetes not-ready endpoint. A disabled instance must never receive
	// traffic, not even as a fallback when no healthy instance exists.
	//
	// The zero value (false) means "not disabled", so backends that do not
	// track the distinction are treated as enabled, preserving the old
	// behavior. It is deliberately separate from Healthy: Disabled is an
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
	// passed through untouched for the caller to route on.
	Metadata map[string]string
}

// Query is the materialized form of a discovery lookup: the required service
// name plus whatever optional dimensions the caller narrowed on. Callers rarely
// build one by hand — [Resolve] and [NewResolver] take name and a
// variadic [Option] slice and build it internally; backends build one with
// [NewQuery] to turn the Option slice they receive into values they can read.
type Query struct {
	// Name is the logical service name to look up. Required.
	Name string

	// Scheme narrows the result to one transport scheme. It is "" (the default)
	// when the caller passed no [WithScheme], which means "no narrowing — return
	// every scheme the service exposes". When non-empty ("tls", "https", "grpc",
	// ...) a backend returns only endpoints whose [Endpoint.Scheme] matches, per
	// [FilterByScheme]. It lets one service name expose both plain and secure
	// instances and have a caller pick one without filtering client-side.
	Scheme string

	// Tag narrows the lookup to instances carrying tag — a Consul service tag, a
	// registry label, or any backend-specific marker that partitions a service's
	// instances and is part of the registry query (not a generic field on
	// [Endpoint]). It is "" (the default) when the caller passed no [WithTag].
	//
	// Unlike [Query.Scheme], tag has no package-level filter ([FilterByScheme]):
	// tag is part of the registry's own query (Consul's Health().Service takes
	// the tag server-side), so a backend that supports tags honors it in its
	// registry call, and a backend that does not (the static backend, k8s DNS,
	// ...) silently ignores it. That asymmetry is intentional — only dimensions
	// with a generic [Endpoint] representation get a shared filter.
	Tag string
}

// Option narrows a [Query]. The zero-length Option set means "no narrowing":
// resolve every endpoint the service exposes, in every scheme/tag. Each
// exported Option (WithScheme, WithTag, ...) sets one optional
// dimension; future dimensions are added as further WithX functions without
// breaking existing call sites or backends, which is why the lookup is
// name-plus-options rather than a growing positional parameter list.
type Option func(*Query)

// WithScheme narrows the lookup to endpoints whose [Endpoint.Scheme] matches s.
// An empty s is a no-op (it leaves the "any scheme" default), so passing
// conditionally — discovery.WithScheme(cfg.Scheme) — needs no guard. Matching
// follows [FilterByScheme]: the empty scheme and "tcp" are treated as the same
// "plain TCP" scheme, so WithScheme("tcp") selects both "" and "tcp" endpoints.
func WithScheme(s string) Option {
	return func(q *Query) {
		if s != "" {
			q.Scheme = s
		}
	}
}

// WithTag narrows the lookup to instances carrying tag (a Consul service tag, a
// registry label, ...). An empty t is a no-op. Honoring tag is backend-specific
// and has no package-level filter — see [Query.Tag].
func WithTag(t string) Option {
	return func(q *Query) {
		if t != "" {
			q.Tag = t
		}
	}
}

// NewQuery materializes name plus opts into a [Query]. It is the canonical way a
// [Discovery] backend turns the variadic Option slice it receives from
// Resolve into values it can read: q := NewQuery(name, opts...), then use
// q.Name, q.Scheme, q.Tag. name is required but not validated here — a backend that
// needs a non-empty name checks it itself.
func NewQuery(name string, opts ...Option) Query {
	q := Query{Name: name}
	for _, o := range opts {
		o(&q)
	}
	return q
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
// cancellation, trace). The freshness machinery (registry watch, subscription,
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
	// Resolve returns the current snapshot of endpoints for name, narrowed by
	// opts (e.g. [WithScheme]). It may block on the first call for a service
	// (seed fetch); a real backend performs network I/O there, so ctx is the
	// call's deadline/cancellation. A backend that receives a non-empty Scheme
	// option returns only matching endpoints — apply [FilterByScheme] to the
	// raw result to honor that contract uniformly.
	Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)
}

// Resolver is a bound by-name read of one logical service's live endpoint
// snapshot. It pairs a [Discovery] backend with a single service name (plus
// narrowing options) so a consumer can ask "which endpoints for this service?"
// without re-carrying the backend label and name on every call. Endpoint
// selection — which one to use — is deliberately NOT here; it belongs in
// loadbalance, which consumes a Resolver as its endpoint source.
//
// A Resolver always surfaces errors: it is a thin, honest re-read of
// [Discovery.Resolve] (which is a cheap in-memory read of a cache-backed
// backend, since freshness lives inside the backend). Callers that want a
// selection layer feed it to loadbalance.NewPool; callers that want a bare
// snapshot call it directly.
type Resolver func() ([]Endpoint, error)

// NewResolver binds the Discovery backend d to the service name, narrowed by
// opts (e.g. [WithScheme], [WithTag]), and returns a Resolver that re-reads the
// live snapshot on every call. It is the single constructor every
// infrastructure client starter (Redis, MySQL, MongoDB, ...) and
// discovery-aware transport (httpx, the gateway) reuses: the starter injects
// the backend bean its config cites and hands it here together with the
// service name.
//
// It seeds with one explicit [Discovery.Resolve] — a synchronous read of the
// current state that also fails fast when the service is unknown.
//
// It returns (nil, nil) — "discovery not in effect" — when d is nil or name is
// empty or mesh mode is on (a sidecar owns discovery+LB), in which case the
// caller dials its configured address directly. The mesh check reads the
// GS_MESH switch (see [go-spring.org/cloud/mesh.Enabled]); it is folded in here
// so no caller repeats the same gate.
func NewResolver(ctx context.Context, d Discovery, name string, opts ...Option) (Resolver, error) {
	if d == nil || name == "" || mesh.Enabled() {
		return nil, nil
	}
	if _, err := d.Resolve(ctx, name, opts...); err != nil {
		return nil, fmt.Errorf("discovery: resolve %q: %w", name, err)
	}
	return func() ([]Endpoint, error) {
		return d.Resolve(context.Background(), name, opts...)
	}, nil
}

// NewStaticDiscovery returns a [Discovery] that serves the given fixed endpoint
// set for every service name. It is the zero-dependency reference backend: a
// real adapter would talk to a naming service (Nacos, Consul, etcd, Kubernetes,
// ...) and refresh its cache as instances come and go; this one never changes.
// Use it for examples, tests, and single-instance local setups where a live
// registry is not wanted.
func NewStaticDiscovery(eps ...Endpoint) Discovery {
	return &staticBackend{eps: eps}
}

// staticBackend is the in-memory, fixed-snapshot [Discovery] behind
// [NewStaticDiscovery]. It serves the same endpoint set for every name and
// never changes.
type staticBackend struct {
	eps []Endpoint
}

func (b *staticBackend) Resolve(_ context.Context, _ string, opts ...Option) ([]Endpoint, error) {
	return FilterByScheme(append([]Endpoint(nil), b.eps...), NewQuery("", opts...).Scheme), nil
}

// FilterByScheme returns eps restricted to those whose [Endpoint.Scheme] matches
// scheme, or eps unchanged when scheme is "". It is the canonical implementation
// of the Scheme narrowing every backend applies to its raw results before
// returning them, so the matching rule lives in one place.
//
// The empty scheme and "tcp" are treated as equivalent ("plain TCP"), mirroring
// the [Endpoint.Scheme] convention: WithScheme("tcp") therefore selects both
// "" and "tcp" endpoints, and WithScheme("") (no narrowing) returns everything.
// Any other value matches exactly.
func FilterByScheme(eps []Endpoint, scheme string) []Endpoint {
	if scheme == "" {
		return eps
	}
	want := normalizeScheme(scheme)
	out := eps[:0:0]
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
// without this, WithScheme("tcp") would silently match nothing. Keeping this
// unexported forces all scheme comparisons through [FilterByScheme].
func normalizeScheme(s string) string {
	if s == "" || s == "tcp" {
		return ""
	}
	return s
}
