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

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

// TestNoGlobalResolverRegistration pins the property this package was
// refactored for: the gsdiscovery scheme is NOT registered process-wide, so a
// connection can only be opened through [Client.Dial] — which is what lets the
// discovery directory and the selection policy be per-connection state instead
// of package state. A global registration here would silently resurrect the
// design this replaced.
func TestNoGlobalResolverRegistration(t *testing.T) {
	assert.That(t, resolver.Get(Scheme) == nil).True()
}

// TestOneBalancerNameIsRegistered pins the ONE process-wide registration gRPC
// forces on this package (its balancer registry has no per-connection form), and
// that what sits behind it is the stateless builder — so the single global entry
// carries no configuration that could go stale.
func TestOneBalancerNameIsRegistered(t *testing.T) {
	b := balancer.Get(balancerName)
	if b == nil {
		t.Fatalf("balancer %q must be registered at init", balancerName)
	}
	assert.That(t, b.Name()).Equal(balancerName)
}

// TestAddrEndpointRoundTrip pins the resolver.Address <-> discovery.Endpoint
// attribute bridge: weight, zone and health survive the trip, and the
// connection's selection rides along with them.
func TestAddrEndpointRoundTrip(t *testing.T) {
	sel := mustConnState(t, "grpc:svc", loadbalance.NewManager())
	in := discovery.Endpoint{
		Addr:     "10.0.0.1:8080",
		Weight:   7,
		Healthy:  true,
		Metadata: map[string]string{discovery.MetaKeyZone: "az1"},
	}
	a := addrFromEndpoint(in, sel)
	out := endpointFromAddr(a)
	assert.That(t, out.Addr).Equal(in.Addr)
	assert.That(t, out.Weight).Equal(7)
	assert.That(t, out.Healthy).True()
	assert.That(t, out.Metadata[discovery.MetaKeyZone]).Equal("az1")

	// The selection the picker will read back is the connection's OWN.
	assert.That(t, selectionFromAddr(a)).Same(sel)

	// Degenerate address: no zone metadata, and no selection to recover.
	bare := addrFromEndpoint(discovery.Endpoint{Addr: "10.0.0.2:9090"}, nil)
	assert.That(t, endpointFromAddr(bare).Addr).Equal("10.0.0.2:9090")
	assert.That(t, endpointFromAddr(bare).Weight).Equal(0)
	assert.That(t, selectionFromAddr(bare) == nil).True()
}

// TestPickInfoFrom_HashKeyAndZone verifies the per-request routing hints:
// WithHashKey / WithZone attach to the caller context and surface on the
// loadbalance.PickInfo the strategy consumes.
func TestPickInfoFrom_HashKeyAndZone(t *testing.T) {
	ctx := WithZone(WithHashKey(context.Background(), "user-42"), "az1")
	pi := pickInfoFrom(balancer.PickInfo{Ctx: ctx, FullMethodName: "/svc/M"})
	assert.That(t, pi.HashKey).Equal("user-42")
	assert.That(t, pi.Zone).Equal("az1")

	pi = pickInfoFrom(balancer.PickInfo{FullMethodName: "/svc/M"})
	assert.That(t, pi.HashKey).Equal("")
	assert.That(t, pi.Zone).Equal("")
}

// TestSelectionKeepsBaseStrategyAndRebuilds covers the selection contract of one
// connection: the base strategy (round-robin) is what a rule falls back to (a
// rule naming none leaves it in force), a named rule replaces it, and every
// change REBUILDS the chain's generation — the record in force moves whole, so
// nothing of the replaced selection keeps deciding traffic.
func TestSelectionKeepsBaseStrategyAndRebuilds(t *testing.T) {
	mgr := loadbalance.NewManager()
	sel := mustConnState(t, "grpc:svc", mgr)
	assert.That(t, sel.chain.Selection().Balancer).Equal(loadbalance.RoundRobin) // base, stated

	// A rule that only sets thresholds states the base strategy and replaces the
	// generation, so the new thresholds are the whole state.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: loadbalance.RoundRobin, OutlierThreshold: 4, OutlierSuspendFor: time.Minute}},
	}})
	assert.Number(t, sel.chain.Selection().OutlierThreshold).Equal(4)

	// A named rule replaces the strategy too.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: loadbalance.Weighted, OutlierThreshold: 4}},
	}})
	assert.That(t, sel.chain.Selection().Balancer).Equal(loadbalance.Weighted)
}

// TestSelectionUnknownStrategyKeepsLastGood pins degrade-don't-fail: a rule
// naming a strategy that is not registered leaves the last good generation in
// force instead of taking the connection down (the governance source has no
// error channel, so the log line IS the report).
func TestSelectionUnknownStrategyKeepsLastGood(t *testing.T) {
	mgr := loadbalance.NewManager()
	sel := mustConnState(t, "grpc:svc", mgr)

	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: "no_such_strategy"}},
	}})
	// The refused rebuild built nothing: the base generation still serves, and
	// the record in force still says so.
	assert.That(t, sel.chain.Selection().Balancer).Equal(loadbalance.RoundRobin)
}

// TestSelectionSubscribesPerConnection pins the wiring [Client.Dial] performs:
// the connection's chain is registered under its service's label, so a push
// reaches it — and nothing process-wide is touched, which is why two connections
// (or two containers) never share a policy.
func TestSelectionSubscribesPerConnection(t *testing.T) {
	mgr := loadbalance.NewManager()

	one := mustConnState(t, "grpc:one", mgr)
	two := mustConnState(t, "grpc:two", mgr)

	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:one", Selection: loadbalance.Selection{Balancer: loadbalance.RoundRobin, OutlierThreshold: 7}},
	}})

	assert.Number(t, one.chain.Selection().OutlierThreshold).Equal(7)
	// The other connection's label was not moved: policies are per connection.
	assert.Number(t, two.chain.Selection().OutlierThreshold).Equal(0)
}

// ---------------------------------------------------------------------------
// Picker
// ---------------------------------------------------------------------------

// fakeSubConn satisfies balancer.SubConn by embedding the interface (nil — the
// picker only uses the SubConn as a map key and never calls methods on it;
// grpc's own tests use the same trick because SubConn has an unexported
// enforcement method).
type fakeSubConn struct {
	balancer.SubConn
	addr string
}

// pickerFor builds the picker a connection's gRPC machinery would build: the
// stateless registered builder over READY SubConns whose addresses carry the
// connection's selection — the path the production resolver feeds it.
func pickerFor(t *testing.T, sel *connState, eps ...discovery.Endpoint) balancer.Picker {
	t.Helper()
	ready := map[balancer.SubConn]base.SubConnInfo{}
	for _, ep := range eps {
		ready[&fakeSubConn{addr: ep.Addr}] = base.SubConnInfo{Address: addrFromEndpoint(ep, sel)}
	}
	return gsPickerBuilder{}.Build(base.PickerBuildInfo{ReadySCs: ready})
}

// TestPickerBuild_NoReadySubConns pins the empty-topology behavior: with no
// READY SubConns the picker is an error picker refusing every Pick.
func TestPickerBuild_NoReadySubConns(t *testing.T) {
	p := gsPickerBuilder{}.Build(base.PickerBuildInfo{})
	_, err := p.Pick(balancer.PickInfo{Ctx: context.Background()})
	assert.That(t, err != nil).True()
	assert.That(t, errors.Is(err, balancer.ErrNoSubConnAvailable)).True()
}

// TestPickerBuild_RefusesAddressesWithoutSelection pins the failure mode of a
// connection whose addresses did not come from this package's resolver: there is
// no policy to apply, so the picker refuses rather than picking blind.
func TestPickerBuild_RefusesAddressesWithoutSelection(t *testing.T) {
	sc := &fakeSubConn{addr: "10.0.0.1:80"}
	p := gsPickerBuilder{}.Build(base.PickerBuildInfo{ReadySCs: map[balancer.SubConn]base.SubConnInfo{
		sc: {Address: addrFromEndpoint(discovery.Endpoint{Addr: sc.addr, Healthy: true}, nil)},
	}})
	_, err := p.Pick(balancer.PickInfo{Ctx: context.Background()})
	assert.That(t, errors.Is(err, balancer.ErrNoSubConnAvailable) || err != nil).True()
}

// TestPicker_RotatesAndEvictsFailingInstance covers the core client-side
// behavior: the round-robin picker rotates across READY SubConns, Done(failure)
// feeds the suspension tracker, and after the threshold of consecutive failures
// the failing instance is evicted from eligibility — leaving the healthy one to
// serve every pick.
func TestPicker_RotatesAndEvictsFailingInstance(t *testing.T) {
	mgr := loadbalance.NewManager()
	sel := mustConnState(t, "grpc:svc", mgr)
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: loadbalance.RoundRobin, OutlierThreshold: 5, OutlierSuspendFor: 30 * time.Second}},
	}})

	good, bad := "10.0.0.1:8080", "10.0.0.2:8080"
	picker := pickerFor(t, sel,
		discovery.Endpoint{Addr: good, Healthy: true, Weight: 1},
		discovery.Endpoint{Addr: bad, Healthy: true, Weight: 1},
	)

	ctx := context.Background()
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		seen[r.SubConn.(*fakeSubConn).addr] = true
		r.Done(balancer.DoneInfo{}) // success: no failure state accumulates
	}
	assert.That(t, seen[good] && seen[bad]).True()

	// Drive the bad instance past the threshold: a success on the good instance
	// clears ITS streak, so only the bad one accumulates. After 5 consecutive
	// failures it is suspended, so every subsequent pick lands on the good one.
	for i := 0; i < 12; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		if r.SubConn.(*fakeSubConn).addr == bad {
			r.Done(balancer.DoneInfo{Err: errors.New("boom")})
		} else {
			r.Done(balancer.DoneInfo{})
		}
	}
	for i := 0; i < 8; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		assert.That(t, r.SubConn.(*fakeSubConn).addr).Equal(good)
		r.Done(balancer.DoneInfo{})
	}
}

// countPicks drives n picks through picker and returns how many landed on each
// address. Every pick is completed successfully so no suspension interferes.
func countPicks(t *testing.T, picker balancer.Picker, n int) map[string]int {
	t.Helper()
	counts := map[string]int{}
	p := picker.(*gsPicker)
	for range n {
		r, err := picker.Pick(balancer.PickInfo{Ctx: context.Background()})
		assert.Error(t, err).Nil()
		for addr, sc := range p.byAddr {
			if sc == r.SubConn {
				counts[addr]++
				break
			}
		}
		r.Done(balancer.DoneInfo{})
	}
	return counts
}

// TestPicker_FollowsTheSelectionOnTheNextPick covers the strategy half end to
// end: the connection runs the base strategy (round-robin), a pushed rule takes
// over on the very next pick with no re-dial and no picker rebuild, and a rule
// that stops naming a strategy returns it to the base.
func TestPicker_FollowsTheSelectionOnTheNextPick(t *testing.T) {
	mgr := loadbalance.NewManager()
	sel := mustConnState(t, "grpc:svc", mgr)

	// 9:1 weights: round-robin splits evenly, weighted strongly favors the first.
	picker := pickerFor(t, sel,
		discovery.Endpoint{Addr: "10.0.0.1:80", Healthy: true, Weight: 9},
		discovery.Endpoint{Addr: "10.0.0.2:80", Healthy: true, Weight: 1},
	)

	counts := countPicks(t, picker, 40)
	assert.Number(t, counts["10.0.0.1:80"]).Equal(20)

	// A pushed rule swaps the strategy: the picker reads the new generation.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: loadbalance.Weighted}},
	}})
	counts = countPicks(t, picker, 100)
	assert.That(t, counts["10.0.0.1:80"] > 80).True()

	// Back to the base strategy, stated explicitly (an empty name would be
	// rejected and leave the weighted generation in force).
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: "grpc:svc", Selection: loadbalance.Selection{Balancer: loadbalance.RoundRobin}},
	}})
	counts = countPicks(t, picker, 40)
	assert.Number(t, counts["10.0.0.1:80"]).Equal(20)
}

// ---------------------------------------------------------------------------
// Resolver
// ---------------------------------------------------------------------------

// fakePollDiscovery serves scripted snapshots: each Resolve call returns the
// next entry (nil entries repeat the last). It models the Resolve-only
// discovery contract — freshness is the backend's job, the resolver just
// re-reads.
type fakePollDiscovery struct {
	discovery.Discovery
	snaps [][]discovery.Endpoint
	calls int
}

func (f *fakePollDiscovery) Resolve(_ context.Context, _ string) ([]discovery.Endpoint, error) {
	i := f.calls
	if i >= len(f.snaps) {
		i = len(f.snaps) - 1
	}
	f.calls++
	return f.snaps[i], nil
}

type fakeClientConn struct {
	resolver.ClientConn
	states chan resolver.State
}

func (f *fakeClientConn) UpdateState(s resolver.State) error {
	f.states <- s
	return nil
}

// TestPollLoop_PushesChanges proves the poll loop keeps gRPC's address set
// current: it re-reads the backend snapshot on each tick, pushes only real
// changes, exits on Close (ctx cancellation), and tags every address with the
// connection's selection. The interval is a field of the resolver — per
// connection, like everything else here — which is what makes it testable
// without a package-level seam.
func TestPollLoop_PushesChanges(t *testing.T) {
	d := &fakePollDiscovery{snaps: [][]discovery.Endpoint{
		{{Addr: "10.0.0.1:80"}},                        // seed
		{{Addr: "10.0.0.1:80"}},                        // no-op: must NOT push again
		{{Addr: "10.0.0.1:80"}, {Addr: "10.0.0.2:80"}}, // scale up: pushed
	}}
	cc := &fakeClientConn{states: make(chan resolver.State, 4)}
	sel := mustConnState(t, "grpc:svc", loadbalance.NewManager())

	ctx, cancel := context.WithCancel(context.Background())
	r := &discoveryResolver{
		cc: cc, discovery: d, sel: sel, poll: 5 * time.Millisecond,
		backend: "fake", serviceName: "svc", ctx: ctx, cancel: cancel,
	}
	r.push(d.snaps[0]) // Build seeds before the loop starts
	go r.pollLoop(d.snaps[0])

	got := func() []string {
		st := <-cc.states
		var addrs []string
		for _, a := range st.Addresses {
			addrs = append(addrs, a.Addr)
			// Every address carries THIS connection's selection.
			assert.That(t, selectionFromAddr(a)).Same(sel)
		}
		return addrs
	}
	assert.That(t, got()).Equal([]string{"10.0.0.1:80"})                // seed
	assert.That(t, got()).Equal([]string{"10.0.0.1:80", "10.0.0.2:80"}) // scale-up

	// Close must stop the loop.
	r.Close()
	select {
	case <-time.After(2 * time.Second):
		t.Fatal("pollLoop did not observe context cancellation promptly")
	case <-ctx.Done():
	}
}

// mustConnState is newConnState with the construction error asserted away.
func mustConnState(t *testing.T, label string, mgr *loadbalance.Manager) *connState {
	t.Helper()
	s, err := newConnState(label, mgr)
	if err != nil {
		t.Fatalf("conn state for %q: %v", label, err)
	}
	return s
}
