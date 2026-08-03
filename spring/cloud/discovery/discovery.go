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
// [Discovery] interface and registering one or more fully-built backends via
// [RegisterDiscovery]; every client starter then resolves names through a named
// Discovery without any per-component adaptation. Publishing this process to a
// registry (the provider-side write) is handled by a registry starter such as
// starter-registry-etcd, not by this package.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
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

	// Weight is the load-balancing weight; 0 is treated as the default weight.
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
// build one by hand — [Resolve], [Watch], and [NewResolver] take name and a
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

	// Group narrows the lookup to one registry group, e.g. a Nacos group that
	// partitions a service's instances. It is "" (the default) when the caller
	// passed no [WithGroup]. As with [Query.Tag], group is backend-interpreted
	// with no package-level filter.
	//
	// Note the deployment nuance: in registries like Nacos, group (and
	// namespace) are often bound per backend at construction — one [Discovery]
	// adapter serves one namespace+group — rather than selected per query. Such
	// deployments leave this empty and configure the group on the backend; this
	// field is for the rarer per-query group narrowing.
	Group string
}

// Option narrows a [Query]. The zero-length Option set means "no narrowing":
// resolve every endpoint the service exposes, in every scheme/tag/group. Each
// exported Option (WithScheme, WithTag, WithGroup, ...) sets one optional
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

// WithGroup narrows the lookup to one registry group (a Nacos group, ...). An
// empty g is a no-op. Honoring group is backend-specific and has no
// package-level filter — see [Query.Group] for the per-query vs per-backend
// nuance.
func WithGroup(g string) Option {
	return func(q *Query) {
		if g != "" {
			q.Group = g
		}
	}
}

// NewQuery materializes name plus opts into a [Query]. It is the canonical way a
// [Discovery] backend turns the variadic Option slice it receives from
// Resolve/Watch into values it can read: q := NewQuery(name, opts...), then use
// q.Name, q.Scheme, q.Tag, q.Group. name is required but not validated here — a backend that
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
// Implementations must be safe for concurrent use.
type Discovery interface {
	// Resolve returns the current snapshot of endpoints for name, narrowed by
	// opts (e.g. [WithScheme]). It is called once at cold start, before a client
	// establishes its first connection. A backend that receives a non-empty
	// Scheme option returns only matching endpoints — apply [FilterByScheme] to
	// the raw result to honor that contract uniformly.
	Resolve(ctx context.Context, name string, opts ...Option) ([]Endpoint, error)

	// Watch opens a subscription to name, narrowed by opts (e.g. [WithScheme]),
	// and returns a channel carrying a [WatchResult] each time the service's
	// endpoint set changes. The first result carries the CURRENT set immediately,
	// so a long-lived caller need not Resolve first; later results carry
	// successive full snapshots as the topology changes. As with Resolve, a
	// non-empty Scheme option means the subscription delivers only matching
	// endpoints.
	//
	// The channel closes when the subscription ends — either because ctx is
	// cancelled or because the backend emitted a terminal [WatchResult.Err]; a
	// for-range loop then exits and the caller keeps serving from the last
	// endpoint set it received, since stale addresses are safer than none.
	// Cancelling ctx is the single way to stop the watch and release backend
	// resources: there is no separate Stop method, no Watcher handle to close.
	// Reconnect with backoff, when wanted, is the caller's responsibility, not
	// Watch's.
	Watch(ctx context.Context, name string, opts ...Option) (<-chan WatchResult, error)
}

// WatchResult is one element delivered on a [Discovery.Watch] channel: either a
// fresh full endpoint set for the service, or a terminal error that ended the
// subscription. It is NOT an incremental delta — every Endpoints value is the
// complete current topology, so a caller always has the full picture and never
// needs to merge per-instance changes.
type WatchResult struct {
	// Endpoints is the full current endpoint set for the watched service. It may
	// be empty (a zero-length, non-nil slice) when the service currently has no
	// live instances — that is a valid snapshot, not an error.
	Endpoints []Endpoint

	// Err, when non-nil, is the terminal error that ended the subscription
	// (backend disconnect, auth expiry, ...). The channel closes after this
	// result is delivered; no further results follow.
	Err error
}

// Catalog is an OPTIONAL capability a Discovery backend may implement when it
// can enumerate every service name it knows — the "discover" half of service
// discovery, used by gateways building routes dynamically, governance
// dashboards, and service catalogs.
//
// It is a separate interface rather than a third method on Discovery because
// some backends cannot enumerate at all (DNS, a static adapter, a Kubernetes
// headless Service reached by name). Forcing enumeration onto Discovery would
// make those backends lie with empty lists or panics. Consumers type-assert:
//
//	if c, ok := d.(Catalog); ok { names, _ := c.Services(ctx) }
//
// A backend that supports enumeration but currently knows no services returns
// (nil, nil); a backend that cannot enumerate simply does not implement
// Catalog. ErrUnsupported exists for the rare wrapper that must implement the
// interface yet delegates to a non-Catalog backend.
type Catalog interface {
	// Services returns the names of every service the backend currently knows
	// about. The order is unspecified; callers that need a stable order sort.
	Services(ctx context.Context) ([]string, error)
}

// ErrUnsupported is returned by a Catalog implementation whose delegate cannot
// list service names. Backends that merely lack the capability should not
// implement Catalog at all.
var ErrUnsupported = errors.New("discovery: service enumeration not supported")

// discoveriesMu guards discoveries. It is independent of registrarsMu (in
// registrar.go): the two registries are written only during init and read only
// at client construction, so neither needs to serialize against the other.
var (
	discoveriesMu sync.RWMutex
	discoveries   = map[string]Discovery{}
)

// RegisterDiscovery publishes a fully-built [Discovery] — an adapter already
// bound to a concrete registry (a specific Nacos server/namespace, an etcd
// cluster, a Kubernetes API, ...) — under the label name.
//
// What name is NOT: it is neither a driver/protocol kind ("nacos", "etcd") nor
// a service name ("order-service"). The adapter kind is just whichever
// implementation was constructed; a service name is what callers later pass to
// [Discovery.Watch]. name here is a user-chosen instance label, typically the
// key of a discovery config block (e.g. ${spring.discovery.k8s.<name>}), that a
// client starter cites to pick which Discovery to resolve through.
//
// It is deliberately distinct from "Register" in the service-registration
// sense: publishing a service instance to a registry (the provider-side write,
// handled by a registry starter such as starter-registry-etcd) is a different
// operation from plugging a Discovery adapter into this package.
//
// It panics on empty name, nil Discovery, or a duplicate name — mirroring the
// driver-registry idiom used elsewhere (e.g. starter-go-redis RegisterDriver) —
// so mis-wiring fails loudly at init rather than silently.
func RegisterDiscovery(name string, d Discovery) {
	if name == "" {
		panic("discovery: register with empty name")
	}
	if d == nil {
		panic("discovery: register nil Discovery for " + name)
	}
	discoveriesMu.Lock()
	defer discoveriesMu.Unlock()
	if _, ok := discoveries[name]; ok {
		panic("discovery: Discovery already registered: " + name)
	}
	discoveries[name] = d
}

// GetDiscovery returns the [Discovery] registered under name. It is the single
// lookup clients use at construction time. A wrong name — a typo, a starter not
// compiled in, an OnProperty block that did not fire — must surface as a
// readable misconfiguration rather than an empty client, so the error lists
// every registered Discovery to make the mismatch obvious. Callers that only
// need a presence check test err == nil.
func GetDiscovery(name string) (Discovery, error) {
	discoveriesMu.RLock()
	defer discoveriesMu.RUnlock()
	if d, ok := discoveries[name]; ok {
		return d, nil
	}
	names := make([]string, 0, len(discoveries))
	for k := range discoveries {
		names = append(names, k)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("discovery: no Discovery registered as %q (registered: %v)", name, names)
}

// NewStaticDiscovery returns a [Discovery] that serves the given fixed endpoint
// set for every service name. It is the zero-dependency reference backend: a
// real adapter would talk to a naming service (Nacos, Consul, etcd, Kubernetes,
// ...) and push fresh snapshots through [Discovery.Watch] as instances come and
// go; this one never changes. Use it for examples, tests, and single-instance
// local setups where a live registry is not wanted.
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

func (b *staticBackend) Watch(ctx context.Context, _ string, opts ...Option) (<-chan WatchResult, error) {
	eps := FilterByScheme(append([]Endpoint(nil), b.eps...), NewQuery("", opts...).Scheme)
	ch := make(chan WatchResult, 1)
	ch <- WatchResult{Endpoints: eps}
	go func() {
		<-ctx.Done()
		close(ch)
	}()
	return ch, nil
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
// bucket so they match each other; every other scheme is left as-is. Keeping
// this unexported forces all scheme comparisons through [FilterByScheme].
func normalizeScheme(s string) string {
	if s == "" || s == "tcp" {
		return ""
	}
	return s
}
