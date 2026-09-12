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
// framework-neutral [go-spring.org/cloud/loadbalance] strategies into gRPC's
// own resolver + balancer machinery. It is the reference RPC integration called
// for by the load-balancing task: a company adapts its naming service once via
// [go-spring.org/cloud/discovery], and any gRPC client picks instances through
// a Go-Spring strategy simply by dialing a "gsdiscovery:///<service>" target
// with the matching service config — no per-client wiring.
//
// Two grpc pieces are provided:
//
//   - a resolver.Builder (scheme "gsdiscovery") that turns a discovery backend
//     into the stream of address snapshots gRPC consumes, so instances coming
//     and going are reflected in real time (task 2.2, Watch side);
//   - a base.PickerBuilder per strategy that selects among the READY SubConns
//     using a loadbalance.Balancer and an suspension loadbalance.Tracker, so a
//     failing-but-still-registered instance is evicted and later readmitted
//     (task 2.2, breaker side).

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

// Scheme is the target scheme handled by the discovery-backed resolver. Dial
// "gsdiscovery:///<service>" to resolve <service> through the discovery backend
// bean named "default", or "gsdiscovery://<backend>/<service>" to select another
// named backend bean.
const Scheme = "gsdiscovery"

// discoveryBackends is the label -> backend directory the resolver builder
// resolves "gsdiscovery://<backend>/<service>" targets against. It is the
// container's named discovery.Discovery beans, captured once at wiring time by
// newDiscoveryBackendsHook (see starter.go) — or by a non-container caller via
// [SetDiscoveryBackends] — so target resolution never consults process-global
// state at runtime. An atomic.Value keeps the read lock-free on the RPC dial
// path.
var discoveryBackends atomic.Value // map[string]discovery.Discovery

// SetDiscoveryBackends installs backends (bean name = label) as the directory
// the gsdiscovery resolver resolves target labels against. The starter's wiring
// calls this once at assembly time; a program that dials gsdiscovery targets
// outside the container (no gs.Run) calls it directly instead. Calling it again
// replaces the directory wholesale.
func SetDiscoveryBackends(backends map[string]discovery.Discovery) {
	discoveryBackends.Store(backends)
}

// lookupDiscoveryBackend returns the backend registered under label.
func lookupDiscoveryBackend(label string) (discovery.Discovery, bool) {
	v, _ := discoveryBackends.Load().(map[string]discovery.Discovery)
	if v == nil {
		return nil, false
	}
	d, ok := v[label]
	return d, ok
}

// backendLabels lists every registered backend label, sorted, for error
// messages.
func backendLabels() []string {
	v, _ := discoveryBackends.Load().(map[string]discovery.Discovery)
	labels := make([]string, 0, len(v))
	for k := range v {
		labels = append(labels, k)
	}
	sort.Strings(labels)
	return labels
}

// clientSuspensionLabel is the governance resource label the pre-registered
// balancers resolve their suspension policy from. It deliberately names no
// service: these balancers are process-wide and shared by every client that
// selects them through service config, so their only sensible policy is the
// process-wide default — govern.default.* — which is exactly what a label
// matching no rule resolves to. Write govern.rules[N].resources=grpc:client to
// target them explicitly.
const clientSuspensionLabel = "grpc:client"

// builtinStrategies are the balancer strategies pre-registered under their
// gRPC names at init, so a client can select one purely through service
// config, e.g. [LoadBalancingConfig](loadbalance.RoundRobin).
var builtinStrategies = []string{
	loadbalance.RoundRobin,
	loadbalance.LeastConn,
	loadbalance.ConsistentHash,
	loadbalance.Weighted,
	loadbalance.ZoneAware,
}

func init() {
	resolver.Register(discoveryResolverBuilder{})

	// One tracker per built-in strategy, all starting DISABLED: suspension is a
	// governance decision now, resolved from the process-wide default and applied
	// in place, so a rule push retunes them all without re-registering a
	// balancer. This is the same "unset means inert" rule every other governed
	// knob follows — a client that wants eviction configures it.
	trackers := make([]*loadbalance.Tracker, 0, len(builtinStrategies))
	for _, s := range builtinStrategies {
		t := loadbalance.NewTracker(loadbalance.TrackerConfig{})
		registerBalancer(BalancerName(s), s, t)
		trackers = append(trackers, t)
	}
	governance.Register(clientSuspensionLabel, func(p resilience.Policy) {
		for _, t := range trackers {
			t.SetConfig(loadbalance.TrackerConfig{
				Threshold:  p.OutlierThreshold,
				SuspendFor: p.OutlierSuspendFor,
			})
		}
	})
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
// strategies are pre-registered in init, driven by governance (see
// clientSuspensionLabel); call this to register a custom name (e.g. per
// service, for isolated suspension state) or a suspension policy of your own
// that governance does not drive. It panics on an unknown strategy or a
// duplicate name, matching gRPC's own balancer.Register contract.
func RegisterBalancer(name, strategy string, tc loadbalance.TrackerConfig) {
	registerBalancer(name, strategy, loadbalance.NewTracker(tc))
}

// registerBalancer registers name over a caller-built tracker, so the built-in
// strategies can keep a handle on theirs and retune it when governance changes.
func registerBalancer(name, strategy string, tracker *loadbalance.Tracker) {
	bal, err := loadbalance.New(strategy)
	if err != nil {
		panic("starter-grpc: " + err.Error())
	}
	pb := &gsPickerBuilder{bal: bal, tracker: tracker}
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

// gsPickerBuilder holds the strategy and suspension tracker shared across picker
// rebuilds so round-robin cursors, least-conn counts and suspension windows
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
	candidates := p.tracker.Allows(p.eps)

	ep, err := p.bal.Pick(candidates, pickInfoFrom(info))
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
			p.bal.Complete(ep, di.Err)
			p.tracker.Record(ep.Addr, di.Err == nil)
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

	d, ok := lookupDiscoveryBackend(backend)
	if !ok {
		return nil, errutil.Explain(nil, "grpc: discovery backend %q not found (no discovery.Discovery bean with this name; registered: %v)", backend, backendLabels())
	}

	ctx, cancel := context.WithCancel(context.Background())
	r := &discoveryResolver{cc: cc, d: d, backend: backend, service: service, ctx: ctx, cancel: cancel}

	// Seed the client with an initial snapshot before starting the poll loop so
	// the first RPC does not race an empty address list.
	eps, err := d.Resolve(ctx, service)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("loadbalance: resolve %q via %q: %w", service, backend, err)
	}
	r.push(eps)
	go r.pollLoop(eps)
	return r, nil
}

type discoveryResolver struct {
	cc      resolver.ClientConn
	d       discovery.Discovery
	backend string
	service string
	ctx     context.Context
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

// pollInterval is how often the resolver re-reads the backend snapshot.
// Backends keep their caches fresh internally, so this bounds only the
// propagation of a change into gRPC's address list. It is a var so tests can
// shrink it.
var pollInterval = 10 * time.Second

// pollLoop keeps the resolved address set current for the lifetime of the
// resolver by re-reading the backend snapshot on pollInterval. A failed read
// keeps the last snapshot — stale addresses are safer than none — and the next
// tick retries; the loop exits only when r.ctx is cancelled, i.e. on Close (or
// clientConn shutdown).
func (r *discoveryResolver) pollLoop(seed []discovery.Endpoint) {
	last := epsKey(seed)
	t := time.NewTicker(pollInterval)
	defer t.Stop()
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-t.C:
			eps, err := r.d.Resolve(r.ctx, r.service)
			if err != nil {
				log.Errorf(r.ctx, log.TagAppDef, "grpc resolver resolve %q via %q failed, keeping last snapshot: %v",
					r.service, r.backend, err)
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
