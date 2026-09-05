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

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/resolver"
)

// TestBalancerNameAndServiceConfig pins the naming contract clients dial with:
// strategy -> "gs_<strategy>" and the service-config JSON that selects it.
func TestBalancerNameAndServiceConfig(t *testing.T) {
	assert.That(t, BalancerName(loadbalance.RoundRobin)).Equal("gs_" + loadbalance.RoundRobin)
	assert.That(t, LoadBalancingConfig(loadbalance.LeastConn)).
		Equal(`{"loadBalancingConfig":[{"gs_least_conn":{}}]}`)
}

// TestAddrEndpointRoundTrip pins the resolver.Address <-> discovery.Endpoint
// attribute bridge: weight, zone and health survive the trip, and an address
// with no attributes degrades to a bare endpoint.
func TestAddrEndpointRoundTrip(t *testing.T) {
	in := discovery.Endpoint{
		Addr:     "10.0.0.1:8080",
		Weight:   7,
		Healthy:  true,
		Metadata: map[string]string{loadbalance.DefaultZoneKey: "az1"},
	}
	out := endpointFromAddr(addrFromEndpoint(in))
	assert.That(t, out.Addr).Equal(in.Addr)
	assert.That(t, out.Weight).Equal(7)
	assert.That(t, out.Healthy).True()
	assert.That(t, out.Metadata[loadbalance.DefaultZoneKey]).Equal("az1")

	// Degenerate address: no attributes, no zone metadata.
	bare := endpointFromAddr(addrFromEndpoint(discovery.Endpoint{Addr: "10.0.0.2:9090"}))
	assert.That(t, bare.Addr).Equal("10.0.0.2:9090")
	assert.That(t, bare.Weight).Equal(0)
	assert.That(t, bare.Metadata == nil || len(bare.Metadata) == 0).True()
}

// TestPickInfoFrom_HashKeyAndZone verifies the per-request routing hints:
// WithHashKey / WithZone attach to the caller context and surface on the
// loadbalance.PickInfo the strategy consumes.
func TestPickInfoFrom_HashKeyAndZone(t *testing.T) {
	ctx := WithZone(WithHashKey(context.Background(), "user-42"), "az1")
	pi := pickInfoFrom(balancer.PickInfo{Ctx: ctx, FullMethodName: "/svc/M"})
	assert.That(t, pi.HashKey).Equal("user-42")
	assert.That(t, pi.Zone).Equal("az1")

	// Without hints and without a context: empty pick info.
	pi = pickInfoFrom(balancer.PickInfo{FullMethodName: "/svc/M"})
	assert.That(t, pi.HashKey).Equal("")
	assert.That(t, pi.Zone).Equal("")
}

// fakeSubConn satisfies balancer.SubConn by embedding the interface (nil —
// the picker only uses the SubConn as a map key and never calls methods on
// it; grpc's own tests use the same trick because SubConn has an unexported
// enforcement method).
type fakeSubConn struct {
	balancer.SubConn
	addr string
}

// newTestPickerBuilder builds a picker over a round-robin strategy with the
// default suspension policy (5 consecutive failures evict for a long window).
func newTestPickerBuilder(t *testing.T) *gsPickerBuilder {
	t.Helper()
	bal, err := loadbalance.New(loadbalance.RoundRobin)
	assert.That(t, err).Nil()
	return &gsPickerBuilder{bal: bal, tracker: loadbalance.NewTracker(defaultTracker)}
}

// TestPickerBuild_NoReadySubConns pins the empty-topology behavior: with no
// READY SubConns the picker is an error picker refusing every Pick.
func TestPickerBuild_NoReadySubConns(t *testing.T) {
	p := newTestPickerBuilder(t).Build(base.PickerBuildInfo{})
	_, err := p.Pick(balancer.PickInfo{Ctx: context.Background()})
	assert.That(t, err != nil).True()
	assert.That(t, errors.Is(err, balancer.ErrNoSubConnAvailable)).True()
}

// TestPicker_RotatesAndEvictsFailingInstance covers the core client-side
// behavior without a registry: the round-robin picker rotates across READY
// SubConns, Done(true/false) feeds the suspension tracker, and after the
// threshold of consecutive failures the failing instance is evicted from
// eligibility — leaving the healthy one to serve every pick.
func TestPicker_RotatesAndEvictsFailingInstance(t *testing.T) {
	good, bad := &fakeSubConn{addr: "10.0.0.1:8080"}, &fakeSubConn{addr: "10.0.0.2:8080"}
	pb := newTestPickerBuilder(t)
	picker := pb.Build(base.PickerBuildInfo{ReadySCs: map[balancer.SubConn]base.SubConnInfo{
		good: {Address: addrFromEndpoint(discovery.Endpoint{Addr: good.addr, Healthy: true, Weight: 1})},
		bad:  {Address: addrFromEndpoint(discovery.Endpoint{Addr: bad.addr, Healthy: true, Weight: 1})},
	}})

	// Drain the suspension window so the test does not wait 30s for readmission.
	t.Cleanup(func() {})

	ctx := context.Background()
	seen := map[string]bool{}
	for i := 0; i < 10; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		seen[r.SubConn.(*fakeSubConn).addr] = true
		r.Done(balancer.DoneInfo{}) // success: no failure state accumulates
	}
	// Round-robin rotated across both instances.
	assert.That(t, seen[good.addr] && seen[bad.addr]).True()

	// Drive the bad instance past the suspension threshold: record a failure
	// whenever it is picked (a success on the good instance clears ITS streak,
	// so only the bad one accumulates). After 5 consecutive failures it is
	// suspended, so every subsequent pick lands on the good instance.
	for i := 0; i < 12; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		if r.SubConn.(*fakeSubConn).addr == bad.addr {
			r.Done(balancer.DoneInfo{Err: errors.New("boom")})
		} else {
			r.Done(balancer.DoneInfo{})
		}
	}
	for i := 0; i < 8; i++ {
		r, err := picker.Pick(balancer.PickInfo{Ctx: ctx})
		assert.That(t, err).Nil()
		assert.That(t, r.SubConn.(*fakeSubConn).addr).Equal(good.addr)
		r.Done(balancer.DoneInfo{})
	}
}

// fakePollDiscovery serves scripted snapshots: each Resolve call returns the
// next entry (nil entries repeat the last). It models the Resolve-only
// discovery contract — freshness is the backend's job, the resolver just
// re-reads.
type fakePollDiscovery struct {
	discovery.Discovery
	snaps [][]discovery.Endpoint
	calls int
}

func (f *fakePollDiscovery) Resolve(_ context.Context, _ string, _ ...discovery.Option) ([]discovery.Endpoint, error) {
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
// changes, and exits on Close (ctx cancellation).
func TestPollLoop_PushesChanges(t *testing.T) {
	pollInterval = 5 * time.Millisecond
	defer func() { pollInterval = 10 * time.Second }()

	d := &fakePollDiscovery{snaps: [][]discovery.Endpoint{
		{{Addr: "10.0.0.1:80"}},                        // seed
		{{Addr: "10.0.0.1:80"}},                        // no-op: must NOT push again
		{{Addr: "10.0.0.1:80"}, {Addr: "10.0.0.2:80"}}, // scale up: pushed
	}}
	cc := &fakeClientConn{states: make(chan resolver.State, 4)}

	ctx, cancel := context.WithCancel(context.Background())
	r := &discoveryResolver{cc: cc, d: d, backend: "fake", service: "svc", ctx: ctx, cancel: cancel}
	r.push(d.snaps[0]) // Build seeds before the loop starts
	go r.pollLoop(d.snaps[0])

	got := func() []string {
		st := <-cc.states
		var addrs []string
		for _, a := range st.Addresses {
			addrs = append(addrs, a.Addr)
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
