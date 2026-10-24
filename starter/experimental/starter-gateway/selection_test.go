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

package StarterGateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// TestClientIP pins the hash key the gateway feeds a governed selection: the
// first X-Forwarded-For hop when present (a comma list must yield the ORIGINAL
// client, not a later proxy), else the RemoteAddr host, else RemoteAddr verbatim.
func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.9")
	assert.That(t, clientIP(r)).Equal("203.0.113.7")

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.Header.Set("X-Forwarded-For", " 203.0.113.7 ")
	assert.That(t, clientIP(r2)).Equal("203.0.113.7")

	r3 := httptest.NewRequest("GET", "/", nil)
	r3.RemoteAddr = "192.0.2.10:54321"
	assert.That(t, clientIP(r3)).Equal("192.0.2.10")

	r4 := httptest.NewRequest("GET", "/", nil)
	r4.RemoteAddr = "not-a-hostport"
	assert.That(t, clientIP(r4)).Equal("not-a-hostport")
}

// TestSelectionHashKeyAffinity covers the payoff of making the route's strategy
// governable: the gateway has always passed the client IP as the pick hash key,
// but under the built-in round_robin default that hint was inert. Once a route's
// rule names consistent_hash, the same client sticks to one upstream while
// different clients spread across both — which is the whole reason the key is
// computed on the proxy path.
func TestSelectionHashKeyAffinity(t *testing.T) {
	mgr := loadbalance.NewManager()
	chain := mgr.Chain(context.Background(), gatewaySource(t, "10.0.0.1:80", "10.0.0.2:80"), gatewayLabel("orders"))

	pickFor := func(clientIP string) string {
		ep, err := chain.Pick(context.Background(), loadbalance.PickInfo{HashKey: clientIP})
		assert.Error(t, err).Nil()
		chain.Complete(context.Background(), ep, nil)
		return ep.Addr
	}

	// Default strategy: the hint is ignored, so one client roams both upstreams.
	seen := map[string]bool{}
	for range 8 {
		seen[pickFor("203.0.113.7")] = true
	}
	assert.Number(t, len(seen)).Equal(2)

	// A rule for the route's label REBUILDS the chain — the route's governance
	// path, driven through the manager exactly as a center push drives it — and
	// the new generation makes the hint authoritative.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{{
		Service:   gatewayLabel("orders"),
		Selection: loadbalance.Selection{Balancer: loadbalance.ConsistentHash},
	}}})
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.ConsistentHash)

	pinned := pickFor("203.0.113.7")
	for range 8 {
		assert.That(t, pickFor("203.0.113.7")).Equal(pinned)
	}

	// Distinct clients still spread, so affinity is per key and not "always the
	// same instance".
	used := map[string]bool{}
	for _, ip := range []string{"198.51.100.9", "192.0.2.10", "203.0.113.99", "198.51.100.42"} {
		used[pickFor(ip)] = true
	}
	assert.Number(t, len(used)).Equal(2)
}

// gatewaySource is the endpoint source the proxy feeds a discovered upstream's
// chain: a static endpoint set.
func gatewaySource(t *testing.T, addrs ...string) discovery.Resolver {
	t.Helper()
	eps := make([]discovery.Endpoint, 0, len(addrs))
	for _, a := range addrs {
		eps = append(eps, discovery.Endpoint{Addr: a, Healthy: true, Weight: 1})
	}
	return func(context.Context) ([]discovery.Endpoint, error) { return eps, nil }
}

// TestClientIPHeadersAreNotTheOnlySource keeps the helper honest for a request
// that carries neither header nor a parseable RemoteAddr.
func TestClientIPHeadersAreNotTheOnlySource(t *testing.T) {
	var r http.Request
	r.RemoteAddr = ""
	assert.That(t, clientIP(&r)).Equal("")
}
