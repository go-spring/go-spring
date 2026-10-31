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

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/testing/assert"
)

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

// counts tallies how many picks landed on each address over n calls.
func counts(t *testing.T, b Balancer, set []discovery.Endpoint, info PickInfo, n int) map[string]int {
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
	return NewManager()
}

// selectionFor reads what label resolves to right now, through the same
// resolution the manager runs when it arms and rebuilds chains.
func selectionFor(m *Manager, label string) Selection {
	return resolveSelection(m.snap.Load(), label)
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
func versionPrefix(v string) matcher.Selector {
	return matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: v}}
}

// mustChain is [Manager.Chain] with the construction error asserted away: a
// chain armed from the base or a valid selection cannot fail to build.
func mustChain(t *testing.T, m *Manager, ctx context.Context, src discovery.Resolver, label string) *Chain {
	t.Helper()
	c, err := m.Chain(ctx, src, label)
	if err != nil {
		t.Fatalf("chain for %q: %v", label, err)
	}
	return c
}

// newBalancer builds the strategy a selection names, through the one entry
// production uses, and fails the test if it cannot be built.
func newBalancer(t *testing.T, s Selection) Balancer {
	t.Helper()
	b, err := New(s)
	if err != nil {
		t.Fatalf("balancer %q: %v", s.Balancer, err)
	}
	return b
}
