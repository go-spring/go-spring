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

package httpx

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"

	"go-spring.org/cloud/governance/dataplane/inbound/admission"

	"go-spring.org/cloud/governance"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/capability/traffic"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// Direct (non-gs) construction must now supply the governance authorities: they
// are REQUIRED because the packages that own them register them, so the
// container always provides them. A test that calls the constructor directly
// passes the same unarmed ones the container would.
var (
	testMgr   = resilience.NewManager(resilience.NewDefaultDriver(nil))
	testInj   = clientfault.NewManager()
	testLbMgr = mustLbManager()
)

// mustLbManager builds a manager over the built-in strategies alone — the
// authority the container would build with no factory bean contributed, which is
// the only shape a direct (non-gs) caller has. The constructor cannot fail for
// that input.
func mustLbManager() *loadbalance.Manager {
	m := loadbalance.NewManager()
	return m
}

// stubDiscovery serves a fixed endpoint set.
type stubDiscovery struct{ eps []discovery.Endpoint }

func (s stubDiscovery) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	return s.eps, nil
}

// recordRT records the host of every request it sees and returns a canned status.
type recordRT struct {
	mu     sync.Mutex
	hosts  []string
	status int
}

func (r *recordRT) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.hosts = append(r.hosts, req.URL.Host)
	r.mu.Unlock()
	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: http.NoBody, Header: http.Header{}}, nil
}

func TestNewTransport_DirectMode(t *testing.T) {
	rec := &recordRT{}
	rt, closeFn, err := NewTransport(Config{Base: rec}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	req, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/ping", nil)
	_, err = rt.RoundTrip(req)
	assert.That(t, err).Nil()
	// Direct mode leaves the host untouched.
	assert.That(t, rec.hosts).Equal([]string{"127.0.0.1:8080"})
}

func TestNewTransport_AddrPinsHost(t *testing.T) {
	rec := &recordRT{}
	rt, closeFn, err := NewTransport(Config{Addr: "10.9.8.7:80", Base: rec}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// Even when the generated client sets a different (or empty) Target, direct
	// Addr mode pins every request to the configured address.
	req, _ := http.NewRequest(http.MethodGet, "http://ignored-target/ping", nil)
	_, err = rt.RoundTrip(req)
	assert.That(t, err).Nil()
	assert.That(t, rec.hosts).Equal([]string{"10.9.8.7:80"})
}

func TestNewTransport_DiscoveryRewritesHost(t *testing.T) {
	backend := stubDiscovery{eps: []discovery.Endpoint{
		{Addr: "10.0.0.1:9000", Healthy: true},
		{Addr: "10.0.0.2:9000", Healthy: true},
	}}

	rec := &recordRT{}
	rt, closeFn, err := NewTransport(Config{
		ServiceName: "user-svc",
		Discovery:   backend,
		Base:        rec,
	}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	for range 4 {
		req, _ := http.NewRequest(http.MethodGet, "http://user-svc/ping", nil)
		_, err = rt.RoundTrip(req)
		assert.That(t, err).Nil()
	}
	// Round-robin over the two discovered endpoints, never the service name.
	assert.That(t, rec.hosts).Equal([]string{
		"10.0.0.1:9000", "10.0.0.2:9000", "10.0.0.1:9000", "10.0.0.2:9000",
	})
}

func TestNewTransport_FailFast(t *testing.T) {
	// A discovery backend whose seed Resolve fails -> fail fast.
	_, _, err := NewTransport(Config{ServiceName: "x", Discovery: errorDiscovery{}}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.Error(t, err).Matches("resolve")
}

// TestGovernSelection_RebuildsThroughTheChain pins the transport's governance
// wiring: a discovery-routed transport builds a chain registered under the
// service's label, so a rule that moves the selection REBUILDS that chain, and
// the live transport keeps serving across the rebuild. (The mapping from a
// governance rule to a Selection, and the strategy-swap behaviour itself, are
// loadbalance's concern, covered by that package's tests.)
func TestGovernSelection_RebuildsThroughTheChain(t *testing.T) {
	mgr := mustLbManager()
	rec := &recordRT{}
	rt, closeFn, err := NewTransport(Config{
		ServiceName: "orders",
		Discovery:   stubDiscovery{eps: []discovery.Endpoint{{Addr: "10.0.0.1:9000", Healthy: true}}},
		Base:        rec,
	}, testCenter(testMgr, testInj, mgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// The base chain routes to the discovered endpoint.
	req, _ := http.NewRequest(http.MethodGet, "http://orders/ping", nil)
	_, err = rt.RoundTrip(req)
	assert.That(t, err).Nil()
	assert.That(t, rec.hosts[0]).Equal("10.0.0.1:9000")

	// A rule for this service's label rebuilds the chain underneath the live
	// transport; the next request is served by the new generation.
	mgr.Apply(context.Background(), loadbalance.Settings{Default: loadbalance.Selection{
		Balancer:         loadbalance.ConsistentHash,
		ConsistentHash:   loadbalance.ConsistentHashParams{Replicas: 100},
		OutlierThreshold: 3,
	}})

	req2, _ := http.NewRequest(http.MethodGet, "http://orders/ping", nil)
	_, err = rt.RoundTrip(req2)
	assert.That(t, err).Nil()
	assert.Number(t, len(rec.hosts)).Equal(2)
	assert.That(t, rec.hosts[1]).Equal("10.0.0.1:9000")
}

// errorDiscovery always fails Resolve, to exercise the seed fail-fast.
type errorDiscovery struct{}

func (errorDiscovery) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	return nil, errors.New("resolve boom")
}

func TestNewTransport_ResilienceBreakerFastFails(t *testing.T) {
	rec := &recordRT{status: http.StatusInternalServerError}
	rt, closeFn, err := NewTransport(Config{
		ResilienceDriver:  resilience.NewDefaultDriver(nil),
		ResilienceBreaker: resilience.BreakerPolicy{ErrorThreshold: 2},
		Base:              rec,
	}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// Drive consecutive 5xx failures to trip the breaker.
	for range 2 {
		req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
		_, _ = rt.RoundTrip(req)
	}
	before := len(rec.hosts)

	// Once open, the breaker rejects before reaching the base transport.
	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	_, err = rt.RoundTrip(req)
	assert.Error(t, err).Matches("circuit")
	assert.That(t, len(rec.hosts)).Equal(before) // base was not hit again
}

// headerRT records whether a given header was present on the request it saw.
type headerRT struct {
	seen      bool
	headerKey string
}

func (h *headerRT) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get(h.headerKey) != "" {
		h.seen = true
	}
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: http.Header{}}, nil
}

func TestNewTransport_InjectsLoadTestMarker(t *testing.T) {
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	rec := &headerRT{headerKey: "X-Traffic"}
	rt, closeFn, err := NewTransport(Config{Base: rec}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// Plain ctx: no marker header on the wire.
	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	_, _ = rt.RoundTrip(req)
	assert.That(t, rec.seen).False()

	// Load-test ctx: the traffic layer injects the marker header.
	req2, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	req2 = req2.WithContext(prop.WithKind(context.Background(), traffic.KindLoadTest))
	_, _ = rt.RoundTrip(req2)
	assert.That(t, rec.seen).True()
}

func TestNewTransport_WrapTransportIsOutermost(t *testing.T) {
	// WrapTransport sits OUTSIDE the traffic layer (the outermost seam), so it
	// sees the request before traffic injects the load-test header — but it can
	// still detect load-test traffic directly via ctx, and the inner base still
	// receives the injected header. This proves the user seam wraps the whole
	// built-in stack and can both observe (via ctx) and delegate.
	var wrapperRan, wrapperSawHeaderBeforeTraffic, wrapperSawLoadTestCtx bool
	prop := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	base := &headerRT{headerKey: "X-Traffic"}
	rt, closeFn, err := NewTransport(Config{
		Base: base,
		WrapTransport: func(inner http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(req *http.Request) (*http.Response, error) {
				wrapperRan = true
				// Outermost: header not yet injected by the traffic layer below.
				wrapperSawHeaderBeforeTraffic = req.Header.Get("X-Traffic") != ""
				// ...but the ctx is already tagged, so a wrapper can always tell.
				wrapperSawLoadTestCtx = traffic.IsLoadTest(req.Context(), prop)
				return inner.RoundTrip(req)
			})
		},
	}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	req = req.WithContext(prop.WithKind(context.Background(), traffic.KindLoadTest))
	_, err = rt.RoundTrip(req)
	assert.That(t, err).Nil()
	assert.That(t, wrapperRan).True()
	assert.That(t, wrapperSawHeaderBeforeTraffic).False() // traffic runs below the wrapper
	assert.That(t, wrapperSawLoadTestCtx).True()          // ctx is readable at any layer
	assert.That(t, base.seen).True()                      // inner base got the injected header
}

// roundTripFunc adapts a function into an http.RoundTripper for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestNewTransport_WrapExecReplacesDefault(t *testing.T) {
	rec := &recordRT{}
	called := false
	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	rt, closeFn, err := NewTransport(Config{
		Base:     rec,
		Executor: mgr.ExecutorFor("test-service"),
		WrapExec: func(e chain.Executor) chain.Executor {
			called = true
			return e
		},
	}, testCenter(testMgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	assert.That(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	assert.That(t, called).True()
}

// TestNewTransport_ResilienceFollowsManager proves the default executor comes
// from the injected manager — the same [resilience.Manager.ExecutorFor] path
// every other client starter takes — so arming that manager hot-swaps the policy
// behind a transport that was already built, with no transport rebuild.
func TestNewTransport_ResilienceFollowsManager(t *testing.T) {
	mgr := resilience.NewManager(resilience.NewDefaultDriver(nil))
	rec := &recordRT{status: http.StatusInternalServerError}
	rt, closeFn, err := NewTransport(Config{Addr: "svc:80", Base: rec}, testCenter(mgr, testInj, testLbMgr), testProp())
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// Unarmed: the manager's executor is a transparent pass-through, so every
	// call reaches the base transport (a 5xx surfaces as an adapter error).
	for range 3 {
		req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
		_, _ = rt.RoundTrip(req)
	}
	before := len(rec.hosts)
	assert.That(t, before).Equal(3)

	// Arm the manager with an error-threshold breaker for the label and push it,
	// exactly as the center does on a config change.
	mgr.ApplyBreaker(resilience.BreakerSettings{Default: resilience.BreakerPolicy{ErrorThreshold: 2}})

	// Two consecutive 5xx now trip the breaker; the third call is rejected
	// before it reaches the base transport — no rebuild in between.
	for range 2 {
		req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
		_, _ = rt.RoundTrip(req)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	_, err = rt.RoundTrip(req)
	assert.Error(t, err).Matches("circuit")
	assert.That(t, len(rec.hosts)).Equal(before + 2)
}

// Service derivation: explicit Service wins; otherwise service-name before
// the direct address, so the label stays stable across addressing-mode switches.
func TestConfigServiceDerivation(t *testing.T) {
	assert.That(t, Config{ServiceName: "user-svc"}.service()).Equal("http:user-svc")
	assert.That(t, Config{Addr: "10.0.0.1:8080", ServiceName: "user-svc"}.service()).Equal("http:user-svc")
	assert.That(t, Config{Addr: "10.0.0.1:8080"}.service()).Equal("http:10.0.0.1:8080")
	assert.That(t, Config{Service: "custom", Addr: "10.0.0.1:8080"}.service()).Equal("custom")
}

// testCenter bundles the trio of test authorities into the one center bean
// NewTransport takes.
func testCenter(mgr *resilience.Manager, inj *clientfault.Manager, lb *loadbalance.Manager) *governance.Runtime {
	ctr := &governance.Runtime{
		Resilience:  mgr,
		LoadBalance: lb,
		Admission:   admission.NewManager(),
		ClientFault: inj,
		ServerFault: serverfault.NewManager(),
		Directory:   discovery.NewManager(nil),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
	}
	if _, err := loadbalanceplane.NewCenter(context.Background(), ctr.LoadBalance, source.NewPushSource(loadbalance.Settings{})); err != nil {
		panic(err)
	}
	if _, err := admissionplane.NewCenter(context.Background(), ctr.Admission, source.NewPushSource(admission.Settings{})); err != nil {
		panic(err)
	}
	return ctr
}

// testProp returns the default load-test convention for transport assembly.
func testProp() traffic.Propagator {
	return traffic.NewDefaultPropagator(traffic.DefaultBinding())
}
