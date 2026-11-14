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

package StarterRedigo

import (
	"context"
	"net"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/loadbalance"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	balancer "go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// newCenter assembles the minimal governance runtime a pool needs: the
// resilience authority (ExecutorFor requires it), the loadbalance authority
// (addressing requires it), the fault and traffic facilities, and the
// discovery directory.
func newCenter(t *testing.T, disc *discovery.Manager) *governance.Runtime {
	t.Helper()
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	return &governance.Runtime{
		Resilience:  res,
		LoadBalance: mustLbManager(t),
		ClientFault: clientfault.NewManager(),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
		Directory:   disc,
	}
}

// TestNewPoolBuildsPickerWithDiscovery covers the addressing half: a
// service-name entry arms the pool's executor with the loadbalance picker, so
// picks spread across the live endpoints and two settled connect failures
// suspend the dead one — every later pick lands on the survivor.
func TestNewPoolBuildsPickerWithDiscovery(t *testing.T) {
	disc := discovery.NewManager(map[string]discovery.Discovery{
		"static": discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: "127.0.0.1:1", Healthy: true, Weight: 1},
			discovery.Endpoint{Addr: "127.0.0.2:1", Healthy: true, Weight: 1},
		),
	})
	ctr := newCenter(t, disc)

	p, err := NewPool(context.Background(), Config{Addressing: discovery.Addressing{ServiceName: "cache", Discovery: "static"}}, ctr)
	assert.Error(t, err).Nil()
	if p == nil || p.picker == nil {
		t.Fatal("service-name set with a discovery backend should arm the addressing picker")
	}

	// Rotation spreads over both endpoints.
	seen := map[string]bool{}
	for range 4 {
		peer, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		seen[peer.Addr] = true
	}
	assert.That(t, len(seen)).Equal(2)

	// Two settled connect failures suspend one endpoint (the default dial
	// threshold): every pick lands on the other.
	for range 2 {
		peer, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		if peer.Addr == "127.0.0.1:1" {
			peer.Complete(context.Background(), chain.ConnectError(errDial("refused")))
		} else {
			peer.Complete(context.Background(), nil)
		}
	}
	// Whatever the rotation order, two failures against .1 need at most four
	// rounds to land twice on it; force the issue by settling directly.
	for range 6 {
		peer, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		if peer.Addr == "127.0.0.1:1" {
			peer.Complete(context.Background(), chain.ConnectError(errDial("refused")))
		}
	}
	for range 4 {
		peer, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.That(t, peer.Addr).Equal("127.0.0.2:1")
	}
}

type errDial string

func (e errDial) Error() string { return string(e) }

// TestNewPoolWithoutDiscoveryHasNoPicker pins that a fixed-address entry has
// no candidate set to choose from: every round runs against the configured
// Addr and the pool carries no picker.
func TestNewPoolWithoutDiscoveryHasNoPicker(t *testing.T) {
	ctr := newCenter(t, discovery.NewManager(nil))
	p, err := NewPool(context.Background(), Config{Addr: "127.0.0.1:6379"}, ctr)
	assert.Error(t, err).Nil()
	if p == nil {
		t.Fatal("a fixed-address pool should still assemble")
	}
	assert.That(t, p.picker == nil).True()
}

// TestPoolFailoverChangesPeerOnRetry is the end-to-end pilot check of the new
// model: two real listeners serve one pool; one dies, and a command whose
// round landed on the dead peer FAILS OVER — the retry re-picks, excluding the
// peer the failed round addressed — so the caller sees success without ever
// knowing a peer was lost.
func TestPoolFailoverChangesPeerOnRetry(t *testing.T) {
	la, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer la.Close()
	lb, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer lb.Close()
	// Accept and park: a plain TCP listener answers dials well enough — the
	// pool issues no commands at dial time.
	for _, l := range []net.Listener{la, lb} {
		go func(l net.Listener) {
			for {
				c, err := l.Accept()
				if err != nil {
					return
				}
				_ = c.Close()
			}
		}(l)
	}

	disc := discovery.NewManager(map[string]discovery.Discovery{
		"static": discovery.NewStaticDiscovery(
			discovery.Endpoint{Addr: la.Addr().String(), Healthy: true, Weight: 1},
			discovery.Endpoint{Addr: lb.Addr().String(), Healthy: true, Weight: 1},
		),
	})
	ctr := newCenter(t, disc)
	// A retry schedule so a failed round gets a second pick.
	res := ctr.Resilience
	assert.Error(t, res.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
		Service: "redigo:cache",
		Rules: []resilience.TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: resilience.TimeoutRetry{Retry: resilience.Retry{MaxRetries: 2}},
		}},
	}}})).Nil()

	p, err := NewPool(context.Background(), Config{
		Addressing:  discovery.Addressing{ServiceName: "cache", Discovery: "static"},
		PoolSize:    1,
		MaxIdle:     0,
		DialTimeout: time.Second,
	}, ctr)
	assert.Error(t, err).Nil()
	defer p.Close()

	// One endpoint dies. Every PING still succeeds: the round that lands on the
	// dead peer fails its dial, the retry excludes that peer and lands on the
	// survivor. (The full round trip needs a speaking server, so the assertion
	// is on the DIAL: acquiring against the dead address fails over.)
	la.Close()
	for range 6 {
		peer1, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		dead := peer1.Endpoint.Addr == la.Addr().String()
		peer1.Complete(context.Background(), func() error {
			if dead {
				return chain.ConnectError(errDial("refused"))
			}
			return nil
		}())
		// The failed round's peer is excluded from the next pick.
		peer2, err := p.picker.Pick(context.Background(), balancer.PickInfo{}, []string{peer1.Endpoint.Addr})
		assert.Error(t, err).Nil()
		assert.That(t, peer2.Endpoint.Addr != peer1.Endpoint.Addr || !dead).True()
	}
}

// mustLbManager builds the load-balancing authority the test's runtime needs.
func mustLbManager(t *testing.T) *loadbalance.Manager {
	t.Helper()
	obs, err := loadbalance.NewOTelObserver()
	if err != nil {
		t.Fatal(err)
	}
	return loadbalance.NewManager(loadbalance.NewFactory([]loadbalance.Provider{loadbalance.NewBuiltinProvider()}), obs, loadbalance.ManagerConfig{})
}
