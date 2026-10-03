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
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/governance"
	"net/http"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// Direct (non-gs) construction must now supply the governance authorities: they
// are REQUIRED because the packages that own them register them, so the
// container always provides them. A test that calls the constructor directly
// passes the same unarmed ones the container would.
var (
	testMgr   = resilience.NewManager(nil)
	testInj   = fault.NewInjector(fault.Configs{}, nil)
	testLbMgr = mustLbManager()
)

// mustLbManager builds a manager over the built-in strategies alone — the
// authority the container would build with no factory bean contributed, which is
// the only shape a direct (non-gs) caller has. The constructor cannot fail for
// that input.
func mustLbManager() *loadbalance.Manager {
	m, err := loadbalance.NewManager(nil)
	if err != nil {
		panic(err)
	}
	return m
}

// stubDiscovery serves a fixed endpoint set.
type stubDiscovery struct{ eps []discovery.Endpoint }

func (s stubDiscovery) Resolve(context.Context, string, ...discovery.Option) ([]discovery.Endpoint, error) {
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
	rt, closeFn, err := NewTransport(Config{Base: rec}, testCenter(testMgr, testInj, testLbMgr), nil)
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
	rt, closeFn, err := NewTransport(Config{Addr: "10.9.8.7:80", Base: rec}, testCenter(testMgr, testInj, testLbMgr), nil)
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
	}, testCenter(testMgr, testInj, testLbMgr), nil)
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
	_, _, err := NewTransport(Config{ServiceName: "x", Discovery: errorDiscovery{}}, testCenter(testMgr, testInj, testLbMgr), nil)
	assert.Error(t, err).Matches("resolve")
}

// TestGovernSelection_BindsThroughManager pins the wiring that replaced the
// removed in-package mapping: a governed transport hands its pool to the
// loadbalance manager, which applies the rule's selection half in place — the
// strategy is swapped without a transport rebuild and the suspension half lands
// on the attached tracker. The mapping itself (SelectionConfig -> Selection) and
// the "an unknown strategy keeps the current one" rule are loadbalance's
// concern now, covered by that package's tests.
func TestGovernSelection_BindsThroughManager(t *testing.T) {
	pool := newTestPool(t, "10.0.0.1:9000", "10.0.0.2:9000")

	// Round-robin (the construction default) alternates between the two.
	first, _ := pool.Pick(loadbalance.PickInfo{HashKey: "k"})
	second, _ := pool.Pick(loadbalance.PickInfo{HashKey: "k"})
	assert.That(t, first.Addr).NotEqual(second.Addr)

	mgr := mustLbManager()
	mgr.Apply(loadbalance.Settings{Enabled: true, Resolve: func(string) loadbalance.Selection {
		return loadbalance.Selection{
			Balancer:          loadbalance.ConsistentHash,
			OutlierThreshold:  3,
			OutlierSuspendFor: time.Second,
		}
	}})
	stop := mgr.Bind(pool, "http:test")
	defer stop()

	// consistent_hash pins one key to one address...
	a, _ := pool.Pick(loadbalance.PickInfo{HashKey: "k"})
	b, _ := pool.Pick(loadbalance.PickInfo{HashKey: "k"})
	assert.That(t, a.Addr).Equal(b.Addr)
	// ...and the suspension half was applied to the same pool.
	assert.That(t, pool.Tracker().Config().Threshold).Equal(3)
	assert.That(t, pool.Tracker().Config().SuspendFor).Equal(time.Second)
}

// newTestPool builds a pool over a fixed endpoint set, with the same disabled
// tracker httpx attaches at construction.
func newTestPool(t *testing.T, addrs ...string) *loadbalance.Pool {
	t.Helper()
	bal := loadbalance.NewRoundRobin()
	eps := make([]discovery.Endpoint, 0, len(addrs))
	for _, a := range addrs {
		eps = append(eps, discovery.Endpoint{Addr: a, Healthy: true})
	}
	return loadbalance.NewPool(
		func() ([]discovery.Endpoint, error) { return eps, nil },
		bal,
	)
}

// errorDiscovery always fails Resolve, to exercise the seed fail-fast.
type errorDiscovery struct{}

func (errorDiscovery) Resolve(context.Context, string, ...discovery.Option) ([]discovery.Endpoint, error) {
	return nil, errors.New("resolve boom")
}

func TestNewTransport_ResilienceBreakerFastFails(t *testing.T) {
	rec := &recordRT{status: http.StatusInternalServerError}
	rt, closeFn, err := NewTransport(Config{
		ResilienceDriver: resilience.NewDefaultDriver(nil),
		ResiliencePolicy: resilience.ClientPolicy{ErrorThreshold: 2},
		Base:             rec,
	}, testCenter(testMgr, testInj, testLbMgr), nil)
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
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	rec := &headerRT{headerKey: "X-LoadTest"}
	rt, closeFn, err := NewTransport(Config{Base: rec}, testCenter(testMgr, testInj, testLbMgr), nil)
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	// Plain ctx: no marker header on the wire.
	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	_, _ = rt.RoundTrip(req)
	assert.That(t, rec.seen).False()

	// Load-test ctx: the traffic layer injects the marker header.
	req2, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	req2 = req2.WithContext(prop.WithLoadTest(context.Background()))
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
	prop, err := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	assert.Error(t, err).Nil()
	base := &headerRT{headerKey: "X-LoadTest"}
	rt, closeFn, err := NewTransport(Config{
		Base: base,
		WrapTransport: func(inner http.RoundTripper) http.RoundTripper {
			return roundTripFunc(func(req *http.Request) (*http.Response, error) {
				wrapperRan = true
				// Outermost: header not yet injected by the traffic layer below.
				wrapperSawHeaderBeforeTraffic = req.Header.Get("X-LoadTest") != ""
				// ...but the ctx is already tagged, so a wrapper can always tell.
				wrapperSawLoadTestCtx = prop.IsLoadTest(req.Context())
				return inner.RoundTrip(req)
			})
		},
	}, testCenter(testMgr, testInj, testLbMgr), nil)
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	req, _ := http.NewRequest(http.MethodGet, "http://svc/ping", nil)
	req = req.WithContext(prop.WithLoadTest(context.Background()))
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
	rt, closeFn, err := NewTransport(Config{
		Base:     rec,
		Executor: resilience.NewManager(nil).ClientExecutorFor("test", "test-service"),
		WrapExec: func(e chain.Executor) chain.Executor {
			called = true
			return e
		},
	}, testCenter(testMgr, testInj, testLbMgr), nil)
	assert.That(t, err).Nil()
	defer func() { _ = closeFn() }()

	req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	assert.That(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	assert.That(t, called).True()
}

// TestNewTransport_ResilienceFollowsManager proves the default executor comes
// from the injected manager — the same [resilience.Manager.ClientExecutorFor] path
// every other client starter takes — so arming that manager hot-swaps the policy
// behind a transport that was already built, with no transport rebuild.
func TestNewTransport_ResilienceFollowsManager(t *testing.T) {
	mgr := resilience.NewManager(nil)
	rec := &recordRT{status: http.StatusInternalServerError}
	rt, closeFn, err := NewTransport(Config{Addr: "svc:80", Base: rec}, testCenter(mgr, testInj, testLbMgr), nil)
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
	mgr.Apply(resilience.Settings{
		Enabled:             true,
		ResolveClientPolicy: func(string) resilience.ClientPolicy { return resilience.ClientPolicy{ErrorThreshold: 2} },
	})

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
func testCenter(mgr *resilience.Manager, inj *fault.Injector, lb *loadbalance.Manager) *governance.Center {
	return governance.NewCenter(governance.Config{}, mgr, lb, inj, nil, nil)
}
