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

package StarterGrpcClient

// This file is the client-side load-balancing adapter that plugs the
// framework-neutral [go-spring.org/cloud/loadbalance] chain into gRPC's own
// resolver + balancer machinery: a per-connection resolver (scheme
// "gsdiscovery") that turns a discovery backend into the stream of address
// snapshots gRPC consumes, and a picker that selects among the READY SubConns
// through a loadbalance.Chain — the same outward surface every other client
// starter picks through. The chain's source is the READY snapshot the picker
// publishes, so gRPC's own SubConn machinery remains the candidate set, and a
// governance push rebuilds the chain's inner generation like anywhere else.
//
// gRPC resolves a resolver by SCHEME and a balancer by NAME, and constructs both
// itself — a builder it finds in a process-wide registry, built from a target
// string and a service config. Nothing in that path accepts an injected value,
// so the state those builders need has to reach them another way. It reaches
// them through the ONE channel gRPC leaves open for per-connection data: the
// resolver attaches the connection's selection state to every
// [resolver.Address] it publishes, and the picker reads it back out of
// [base.PickerBuildInfo]. The builders themselves are therefore stateless, and
// this package keeps no mutable package state at all.
//
// The resolver is registered PER CONNECTION ([grpc.WithResolvers]), not through
// the process-wide [resolver.Register], so the discovery directory a connection
// resolves against travels with the connection that asked for it — see
// [Client.Dial].

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

// Scheme is the target scheme handled by the discovery-backed resolver: a
// connection to "gsdiscovery:///<service>" resolves <service> through the
// discovery backend bean named "default", or "gsdiscovery://<backend>/<service>"
// through another named backend bean. [Client.Dial] builds such a target, so an
// application normally never writes one by hand.
const Scheme = "gsdiscovery"

// balancerName is the one gRPC balancer name this package registers — and it is
// the one thing here that cannot be per-connection. gRPC's balancer registry is a
// package-level map behind [balancer.Register]/[balancer.Get]: a service config
// names a balancer, gRPC looks the name up, and there is no per-connection form
// of the registration (unlike [resolver], which the connection-local
// [grpc.WithResolvers] covers — that asymmetry is the whole reason the resolver
// below is per-connection and this is not). Every gRPC balancer works this way,
// including the built-ins: round_robin, pick_first and the weighted/least-request
// policies all register themselves at init.
//
// What sits behind the name is not state, which is what makes the one global
// entry harmless: the builder is empty, and the selection it applies is the
// connection's own, recovered from the addresses that connection's resolver
// published (see [gsPickerBuilder.Build]).
const balancerName = "gs"

func init() {
	// Register at INIT, as gRPC requires — its own contract is that Register "must
	// only be called during initialization time ... and is not thread-safe", so
	// deferring it to a Client's construction would be wrong as well as pointless.
	// What is registered carries no configuration, so the entry can never go
	// stale.
	balancer.Register(base.NewBalancerBuilder(balancerName, gsPickerBuilder{}, base.Config{HealthCheck: true}))
}

// defaultPollInterval is how often a resolver re-reads the backend snapshot.
// Backends keep their caches fresh internally, so this bounds only the
// propagation of a change into gRPC's address list.
const defaultPollInterval = 10 * time.Second

// connState is ONE connection's endpoint-selection state: a [loadbalance.Chain]
// the connection picks through, plus the live READY endpoint snapshot its picker
// feeds the chain with. It is created by [Client.Dial] per connection — never
// shared, so two connections to different services never see each other's
// policy — and reaches its picker through the resolver's address attributes.
//
// The chain IS the whole selection lifecycle here, exactly as for the other
// client starters: governance refresh rebuilds the chain's inner generation
// (balancer, tracker, filters) whole, and the pick path — [connState.Pick],
// [connState.Complete] — talks to the chain, never to a generation directly.
// There is no Subscribe callback to keep in step: registering the chain with the
// manager (see [Client.Dial]) is the entire wiring.
type connState struct {
	// chain is the connection's outward load-balancing surface, stable for the
	// connection's life. A governance push swaps the generation INSIDE it; this
	// pointer never moves.
	chain *loadbalance.Chain

	// eps is the READY-endpoint snapshot the current picker published. gRPC owns
	// the candidate set (its SubConn machinery, not a discovery resolver), so the
	// chain's source is the closure below reading this snapshot — the one bridge
	// between gRPC's address list and the chain's filter pipeline.
	eps atomic.Pointer[[]discovery.Endpoint]
}

// newConnState builds a connection's selection state: a chain over the
// READY-snapshot source, following label. mgr must be non-nil — the container
// injects the manager bean.
func newConnState(label string, mgr *loadbalance.Manager) *connState {
	s := &connState{}
	src := func() ([]discovery.Endpoint, error) {
		if eps := s.eps.Load(); eps != nil {
			return *eps, nil
		}
		return nil, nil
	}
	s.chain = mgr.Chain(context.Background(), src, label)
	return s
}

// Pick selects one READY endpoint through the chain's generation in force.
func (s *connState) Pick(info balancer.PickInfo) (discovery.Endpoint, error) {
	return s.chain.Pick(pickInfoFrom(info))
}

// Complete settles a pick issued by [connState.Pick].
func (s *connState) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	s.chain.Complete(ctx, ep, err)
}

// publish installs eps as the live candidate snapshot. It is called from the
// picker build path, which runs on gRPC's own serialized balancer goroutine.
func (s *connState) publish(eps []discovery.Endpoint) {
	s.eps.Store(&eps)
}

// ---------------------------------------------------------------------------
// Per-request routing hints
// ---------------------------------------------------------------------------

type ctxKey int

const (
	hashKeyCtxKey ctxKey = iota
	zoneCtxKey
)

// WithHashKey attaches a consistent-hash key to ctx so a consistent_hash
// balancer routes this call (and others sharing the key) to the same instance.
func WithHashKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, hashKeyCtxKey, key)
}

// WithZone attaches the caller's locality to ctx so a zone_aware balancer
// prefers instances advertising the same zone.
func WithZone(ctx context.Context, zone string) context.Context {
	return context.WithValue(ctx, zoneCtxKey, zone)
}

func pickInfoFrom(info balancer.PickInfo) loadbalance.PickInfo {
	var pi loadbalance.PickInfo
	if info.Ctx != nil {
		if v, ok := info.Ctx.Value(hashKeyCtxKey).(string); ok {
			pi.HashKey = v
		}
		if v, ok := info.Ctx.Value(zoneCtxKey).(string); ok {
			pi.Zone = v
		}
	}
	return pi
}

// ---------------------------------------------------------------------------
// Address <-> Endpoint attribute bridge
// ---------------------------------------------------------------------------

type addrAttrKey struct{}

// addrAttr is what this package attaches to every [resolver.Address] it
// publishes: the routing-relevant fields of the discovery [discovery.Endpoint]
// it came from, plus the connection's selection state.
//
// Both travel together because a gRPC address carries exactly one attribute
// value in this version, and because they belong together: the endpoint is what
// the address IS, the selection is which connection asked for it — that second
// half is how a stateless picker learns whose policy to apply, and it is exactly
// the connection's own state, never another's.
//
// It is deliberately free of maps so that resolver.Address.Equal (which compares
// Attributes) stays panic-free. The selection pointer is compared by identity,
// which is what it is: two connections are never the same selection.
type addrAttr struct {
	weight  int
	zone    string
	healthy bool
	sel     *connState
}

func addrFromEndpoint(ep discovery.Endpoint, sel *connState) resolver.Address {
	return resolver.Address{
		Addr: ep.Addr,
		Attributes: attributes.New(addrAttrKey{}, addrAttr{
			weight:  ep.Weight,
			healthy: ep.Healthy,
			zone:    ep.Metadata[loadbalance.DefaultZoneKey],
			sel:     sel,
		}),
	}
}

func endpointFromAddr(a resolver.Address) discovery.Endpoint {
	ep := discovery.Endpoint{Addr: a.Addr}
	if v, ok := addrAttrOf(a); ok {
		ep.Weight = v.weight
		ep.Healthy = v.healthy
		if v.zone != "" {
			ep.Metadata = map[string]string{loadbalance.DefaultZoneKey: v.zone}
		}
	}
	return ep
}

// selectionFromAddr recovers the connection state the resolver attached. It
// returns nil for an address that did not come from this package's resolver,
// which the picker treats as "nothing to select with".
func selectionFromAddr(a resolver.Address) *connState {
	v, ok := addrAttrOf(a)
	if !ok {
		return nil
	}
	return v.sel
}

func addrAttrOf(a resolver.Address) (addrAttr, bool) {
	if a.Attributes == nil {
		return addrAttr{}, false
	}
	v, ok := a.Attributes.Value(addrAttrKey{}).(addrAttr)
	return v, ok
}

// ---------------------------------------------------------------------------
// Picker
// ---------------------------------------------------------------------------

// gsPickerBuilder is the registered picker builder for [balancerName]. It has no
// fields: everything it needs per connection arrives in [base.PickerBuildInfo],
// which is what keeps the one process-wide registration stateless.
type gsPickerBuilder struct{}

func (gsPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	eps := make([]discovery.Endpoint, 0, len(info.ReadySCs))
	byAddr := make(map[string]balancer.SubConn, len(info.ReadySCs))
	var sel *connState
	for sc, sci := range info.ReadySCs {
		ep := endpointFromAddr(sci.Address)
		eps = append(eps, ep)
		byAddr[ep.Addr] = sc
		if sel == nil {
			sel = selectionFromAddr(sci.Address)
		}
	}
	if sel == nil {
		// Addresses not produced by this package's resolver: there is no
		// selection to apply, so refuse rather than pick blind.
		return base.NewErrPicker(errNoSelection)
	}
	// The candidate snapshot the chain's source reads: published once per picker
	// build, so a pick between two builds may see the newer set — never a stale
	// one that still names a SubConn this picker no longer holds.
	sel.publish(eps)
	return &gsPicker{sel: sel, byAddr: byAddr}
}

// errNoSelection reports addresses that carry no connection selection — a
// connection whose resolver is not this package's (a hand-built target, or a
// builder registered without the state attribute).
var errNoSelection = fmt.Errorf("starter-grpc-client: addresses carry no endpoint selection (dial through Client.Dial)")

type gsPicker struct {
	sel    *connState
	byAddr map[string]balancer.SubConn
}

func (p *gsPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	// gRPC has already narrowed the candidates to READY SubConns (dead instances
	// are dropped there — the "kill an instance" path); the chain layers
	// eligibility, breaker-style suspension and drain on top, and its balancer
	// makes the choice.
	ep, err := p.sel.Pick(info)
	if err != nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	sc, ok := p.byAddr[ep.Addr]
	if !ok {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	return balancer.PickResult{
		SubConn: sc,
		Done: func(di balancer.DoneInfo) {
			p.sel.Complete(info.Ctx, ep, di.Err)
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Resolver
// ---------------------------------------------------------------------------

// resolverBuilder resolves "gsdiscovery://<backend>/<service>" targets against a
// discovery directory it HOLDS, rather than against a package-level one: [Client]
// builds it per connection and registers it with [grpc.WithResolvers], so two
// connections may resolve through different directories (and a connection built
// outside the container is simply one whose builder was given a directory
// explicitly).
type resolverBuilder struct {
	dir  *discovery.Manager
	sel  *connState
	poll time.Duration
}

func (resolverBuilder) Scheme() string { return Scheme }

func (b resolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	backend := target.URL.Host
	if backend == "" {
		backend = "default"
	}
	service := target.Endpoint()

	d, ok := b.dir.Get(backend)
	if !ok {
		return nil, errutil.Explain(nil, "grpc: discovery backend %q not found (no discovery.Discovery bean with this name; registered: %v)", backend, b.dir.Labels())
	}

	ctx, cancel := context.WithCancel(context.Background())
	poll := b.poll
	if poll <= 0 {
		poll = defaultPollInterval
	}
	r := &discoveryResolver{cc: cc, discovery: d, sel: b.sel, poll: poll, backend: backend, serviceName: service, ctx: ctx, cancel: cancel}

	// Seed the client with an initial snapshot before starting the poll loop so
	// the first RPC does not race an empty address list.
	eps, err := d.Resolve(ctx, service)
	if err != nil {
		cancel()
		return nil, errutil.Explain(err, "loadbalance: resolve %q via %q", service, backend)
	}
	r.push(eps)
	go r.pollLoop(eps)
	return r, nil
}

type discoveryResolver struct {
	cc          resolver.ClientConn
	discovery   discovery.Discovery
	sel         *connState
	poll        time.Duration
	backend     string
	serviceName string
	ctx         context.Context
	cancel      context.CancelFunc
}

// push converts a discovery snapshot into a gRPC resolver state update, tagging
// every address with the connection's selection so the picker can find it.
func (r *discoveryResolver) push(eps []discovery.Endpoint) {
	addrs := make([]resolver.Address, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, addrFromEndpoint(ep, r.sel))
	}
	// UpdateState errors only report that gRPC ignored an empty/invalid state;
	// there is nothing actionable to do here beyond letting the next snapshot
	// retry, so the error is intentionally not surfaced.
	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

// pollLoop keeps the resolved address set current for the lifetime of the
// resolver by re-reading the backend snapshot on r.poll. A failed read keeps the
// last snapshot — stale addresses are safer than none — and the next tick
// retries; the loop exits only when r.ctx is cancelled, i.e. on Close (or
// clientConn shutdown).
func (r *discoveryResolver) pollLoop(seed []discovery.Endpoint) {
	last := epsKey(seed)
	t := time.NewTicker(r.poll)
	defer t.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-t.C:
			eps, err := r.discovery.Resolve(r.ctx, r.serviceName)
			if err != nil {
				log.Error(r.ctx, log.TagAppDef, err, "grpc resolver resolve failed, keeping last snapshot",
					log.String("service", r.serviceName),
					log.String("resolver", r.backend))
				continue
			}
			key := epsKey(eps)
			if key == last {
				continue // no-op re-delivery must not churn gRPC state
			}
			last = key
			r.push(eps)
		}
	}
}

// epsKey renders a snapshot as a comparable string for change detection.
func epsKey(eps []discovery.Endpoint) string {
	var b strings.Builder
	for _, e := range eps {
		b.WriteString(e.Addr)
		b.WriteByte(';')
	}
	return b.String()
}

// ResolveNow is a no-op: the background watch already keeps the address set
// current, so there is nothing extra to trigger on demand.
func (r *discoveryResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *discoveryResolver) Close() {
	r.cancel()
}
