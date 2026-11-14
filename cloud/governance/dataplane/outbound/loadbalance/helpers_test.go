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

package loadbalance

import (
	"context"
	"testing"

	balancer "go-spring.org/cloud/loadbalance"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/testing/assert"
)

// ep builds a plain endpoint from an address, the unit Record settles.
func ep(addr string) discovery.Endpoint {
	return discovery.Endpoint{Addr: addr}
}

// eps builds a plain endpoint slice from addresses.
func eps(addrs ...string) []discovery.Endpoint {
	out := make([]discovery.Endpoint, len(addrs))
	for i, a := range addrs {
		out[i] = discovery.Endpoint{Addr: a}
	}
	return out
}

// addrs extracts the addresses from an endpoint slice (Endpoint is not
// comparable — it holds a map — so slice assertions run over addresses).
func addrs(eps []discovery.Endpoint) []string {
	out := make([]string, len(eps))
	for i, ep := range eps {
		out[i] = ep.Addr
	}
	return out
}

// suspended reports whether addr is currently suspended and cooling down, so a
// half-open or recovered endpoint reads as not suspended. It probes through
// Available with an endpoint the tracker has never seen: that endpoint is always
// admissible, which keeps the all-suspended fallback from masking the answer.
// suspendedState reads the tracker's stored suspension state directly. Unlike
// [suspended] it does not call Available, so it does not replace the candidate
// set the MinHealthyPercent guard counts within.
func suspendedState(tr *Tracker, addr string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	s := tr.states[addr]
	return s != nil && s.suspended()
}

func suspended(tr *Tracker, addr string) bool {
	for _, ep := range tr.Available(context.Background(), eps(addr, "__probe__")) {
		if ep.Addr == addr {
			return false
		}
	}
	return true
}

// counts tallies how many picks landed on each address over n calls.
func counts(t *testing.T, b balancer.Balancer, set []discovery.Endpoint, info balancer.PickInfo, n int) map[string]int {
	t.Helper()
	m := map[string]int{}
	for range n {
		ep, err := b.Pick(set, info)
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(context.Background(), ep, nil)
	}
	return m
}

// staticSource is a fixed endpoint source for pool tests.
func staticSource(eps ...discovery.Endpoint) discovery.Resolver {
	return func(context.Context) ([]discovery.Endpoint, error) { return eps, nil }
}

// newManager builds a manager over the built-in strategies alone, which is the
// shape a test wants when it exercises selection rather than contributed
// factories.
func newManager(t *testing.T) *Manager {
	t.Helper()
	obs, err := NewOTelObserver()
	if err != nil {
		t.Fatalf("observer: %v", err)
	}
	return NewManager(testFactories(), obs, ManagerConfig{})
}

// selectionFor reads what service resolves to right now, through the same
// resolution the manager runs when it arms and rebuilds chains.
func selectionFor(m *Manager, service string) Selection {
	return findSelection(*m.snap.Load(), service)
}

// versionedSource is a source whose endpoints carry a version, the dimension the
// dynamic selector narrows on.
func versionedSource() discovery.Resolver {
	return staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Metadata: map[string]string{"version": "v2.1"}},
		discovery.Endpoint{Addr: "b", Healthy: true, Metadata: map[string]string{"version": "v1.9"}},
	)
}

// versionPrefix builds the selector a versioned rule narrows with.
func versionPrefix(v string) map[string]matcher.StringMatch {
	return map[string]matcher.StringMatch{"version": {Prefix: v}}
}

// mustChain is [Manager.Chain] with the construction error asserted away: a
// chain armed from the base or a valid selection cannot fail to build.
func mustChain(t *testing.T, m *Manager, ctx context.Context, src discovery.Resolver, service string) *Chain {
	t.Helper()
	c, err := m.Chain(ctx, src, service)
	if err != nil {
		t.Fatalf("chain for %q: %v", service, err)
	}
	return c
}

// testFactories is the built-in strategy set the tests build through.
func testFactories() *Factory {
	return NewFactory([]Provider{NewBuiltinProvider()})
}

// newBalancer builds the strategy a selection names, through the one entry
// production uses, and fails the test if it cannot be built.
func newBalancer(t *testing.T, s Selection) balancer.Balancer {
	t.Helper()
	b, err := testFactories().Create(s)
	if err != nil {
		t.Fatalf("balancer %q: %v", s.Balancer, err)
	}
	return b
}
