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

package StarterGrpc

// This file is the client-side load-balancing adapter that plugs the
// framework-neutral [go-spring.org/spring/loadbalance] strategies into gRPC's
// own resolver + balancer machinery. It is the reference RPC integration called
// for by the load-balancing task: a company adapts its naming service once via
// [go-spring.org/spring/discovery], and any gRPC client picks instances through
// a Go-Spring strategy simply by dialing a "gsdiscovery:///<service>" target
// with the matching service config — no per-client wiring.
//
// Two grpc pieces are provided:
//
//   - a resolver.Builder (scheme "gsdiscovery") that turns a discovery backend
//     into the stream of address snapshots gRPC consumes, so instances coming
//     and going are reflected in real time (task 2.2, Watch side);
//   - a base.PickerBuilder per strategy that selects among the READY SubConns
//     using a loadbalance.Balancer and an ejection loadbalance.Tracker, so a
//     failing-but-still-registered instance is evicted and later readmitted
//     (task 2.2, breaker side).

import (
	"context"
	"fmt"

	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/loadbalance"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

// Scheme is the target scheme handled by the discovery-backed resolver. Dial
// "gsdiscovery:///<service>" to resolve <service> through the default discovery
// backend, or "gsdiscovery://<backend>/<service>" to select a named backend.
const Scheme = "gsdiscovery"

// defaultTracker is the ejection policy attached to the pre-registered
// per-strategy balancers: five consecutive failures evict an instance for 30s
// before a half-open trial. Tune by registering a balancer with RegisterBalancer.
var defaultTracker = loadbalance.TrackerConfig{Threshold: 5, EjectFor: 30_000_000_000} // 30s

func init() {
	resolver.Register(discoveryResolverBuilder{})
	// Pre-register one gRPC balancer per built-in strategy so clients can select
	// it purely through service config, e.g. LoadBalancingConfig(loadbalance.RoundRobin).
	for _, s := range []string{
		loadbalance.RoundRobin,
		loadbalance.LeastConn,
		loadbalance.ConsistentHash,
		loadbalance.Weighted,
		loadbalance.ZoneAware,
	} {
		RegisterBalancer(BalancerName(s), s, defaultTracker)
	}
}

// BalancerName is the gRPC balancer name for a loadbalance strategy, e.g.
// BalancerName(loadbalance.RoundRobin) == "gs_round_robin".
func BalancerName(strategy string) string { return "gs_" + strategy }

// LoadBalancingConfig returns the gRPC service-config JSON that selects the
// balancer for strategy. Pass it to grpc.WithDefaultServiceConfig when dialing.
func LoadBalancingConfig(strategy string) string {
	return fmt.Sprintf(`{"loadBalancingConfig":[{%q:{}}]}`, BalancerName(strategy))
}

// RegisterBalancer registers a gRPC balancer under name that selects instances
// using the loadbalance strategy and evicts failing ones per tc. The built-in
// strategies are pre-registered in init; call this to register a custom name
// (e.g. per service, for isolated ejection state) or a non-default ejection
// policy. It panics on an unknown strategy or a duplicate name, matching gRPC's
// own balancer.Register contract.
func RegisterBalancer(name, strategy string, tc loadbalance.TrackerConfig) {
	bal, err := loadbalance.New(strategy)
	if err != nil {
		panic("starter-grpc: " + err.Error())
	}
	pb := &gsPickerBuilder{bal: bal, tracker: loadbalance.NewTracker(tc)}
	balancer.Register(base.NewBalancerBuilder(name, pb, base.Config{HealthCheck: true}))
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
	pi := loadbalance.PickInfo{Ctx: info.Ctx}
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

type epAttrKey struct{}

// epAttr carries the routing-relevant, comparable fields of a discovery
// Endpoint through a resolver.Address. It is deliberately free of maps so that
// resolver.Address.Equal (which compares Attributes) stays panic-free.
type epAttr struct {
	weight  int
	zone    string
	healthy bool
}

func addrFromEndpoint(ep discovery.Endpoint) resolver.Address {
	a := epAttr{weight: ep.Weight, healthy: ep.Healthy, zone: ep.Metadata[loadbalance.DefaultZoneKey]}
	return resolver.Address{
		Addr:       ep.Addr,
		Attributes: attributes.New(epAttrKey{}, a),
	}
}

func endpointFromAddr(a resolver.Address) discovery.Endpoint {
	ep := discovery.Endpoint{Addr: a.Addr}
	if a.Attributes != nil {
		if v, ok := a.Attributes.Value(epAttrKey{}).(epAttr); ok {
			ep.Weight = v.weight
			ep.Healthy = v.healthy
			if v.zone != "" {
				ep.Metadata = map[string]string{loadbalance.DefaultZoneKey: v.zone}
			}
		}
	}
	return ep
}

// ---------------------------------------------------------------------------
// Picker
// ---------------------------------------------------------------------------

// gsPickerBuilder holds the strategy and ejection tracker shared across picker
// rebuilds so round-robin cursors, least-conn counts and ejection windows
// survive topology changes. A distinct name (see RegisterBalancer) gets its own
// state; sharing a name across services is safe because all state is keyed by
// endpoint address.
type gsPickerBuilder struct {
	bal     loadbalance.Balancer
	tracker *loadbalance.Tracker
}

func (pb *gsPickerBuilder) Build(info base.PickerBuildInfo) balancer.Picker {
	if len(info.ReadySCs) == 0 {
		return base.NewErrPicker(balancer.ErrNoSubConnAvailable)
	}
	eps := make([]discovery.Endpoint, 0, len(info.ReadySCs))
	byAddr := make(map[string]balancer.SubConn, len(info.ReadySCs))
	for sc, sci := range info.ReadySCs {
		ep := endpointFromAddr(sci.Address)
		eps = append(eps, ep)
		byAddr[ep.Addr] = sc
	}
	return &gsPicker{bal: pb.bal, tracker: pb.tracker, eps: eps, byAddr: byAddr}
}

type gsPicker struct {
	bal     loadbalance.Balancer
	tracker *loadbalance.Tracker
	eps     []discovery.Endpoint
	byAddr  map[string]balancer.SubConn
}

func (p *gsPicker) Pick(info balancer.PickInfo) (balancer.PickResult, error) {
	// gRPC has already narrowed p.eps to READY SubConns (dead instances are
	// dropped here — the "kill an instance" path). The tracker layers breaker-
	// style eviction on top for instances that are connectable but failing.
	candidates := p.tracker.Eligible(p.eps)

	r, err := p.bal.Pick(candidates, pickInfoFrom(info))
	if err != nil {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}
	sc, ok := p.byAddr[r.Endpoint.Addr]
	if !ok {
		return balancer.PickResult{}, balancer.ErrNoSubConnAvailable
	}

	addr := r.Endpoint.Addr
	inner := r.Done
	return balancer.PickResult{
		SubConn: sc,
		Done: func(di balancer.DoneInfo) {
			if inner != nil {
				inner(loadbalance.DoneInfo{Err: di.Err})
			}
			p.tracker.Record(addr, di.Err == nil)
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Resolver
// ---------------------------------------------------------------------------

type discoveryResolverBuilder struct{}

func (discoveryResolverBuilder) Scheme() string { return Scheme }

func (discoveryResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	backend := target.URL.Host
	if backend == "" {
		backend = "default"
	}
	service := target.Endpoint()

	d, err := discovery.MustGet(backend)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &discoveryResolver{cc: cc, service: service, cancel: cancel}

	// Seed the client with an initial snapshot before starting the watch so the
	// first RPC does not race an empty address list.
	eps, err := d.Resolve(ctx, service)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("loadbalance: resolve %q via %q: %w", service, backend, err)
	}
	r.push(eps)

	w, err := d.Watch(ctx, service)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("loadbalance: watch %q via %q: %w", service, backend, err)
	}
	r.watcher = w
	go r.watchLoop()
	return r, nil
}

type discoveryResolver struct {
	cc      resolver.ClientConn
	service string
	watcher discovery.Watcher
	cancel  context.CancelFunc
}

// push converts a discovery snapshot into a gRPC resolver state update.
func (r *discoveryResolver) push(eps []discovery.Endpoint) {
	addrs := make([]resolver.Address, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, addrFromEndpoint(ep))
	}
	// UpdateState errors only report that gRPC ignored an empty/invalid state;
	// there is nothing actionable to do here beyond letting the next snapshot
	// retry, so the error is intentionally not surfaced.
	_ = r.cc.UpdateState(resolver.State{Addresses: addrs})
}

func (r *discoveryResolver) watchLoop() {
	for {
		eps, err := r.watcher.Next()
		if err != nil {
			return
		}
		r.push(eps)
	}
}

// ResolveNow is a no-op: the background watch already keeps the address set
// current, so there is nothing extra to trigger on demand.
func (r *discoveryResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (r *discoveryResolver) Close() {
	r.cancel()
	if r.watcher != nil {
		_ = r.watcher.Stop()
	}
}
