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
	"net/http"
	"sync"
	"testing"

	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// stubDiscovery serves a fixed endpoint set and a watcher that never updates.
type stubDiscovery struct{ eps []discovery.Endpoint }

func (s stubDiscovery) Resolve(context.Context, string) ([]discovery.Endpoint, error) {
	return s.eps, nil
}

func (s stubDiscovery) Watch(context.Context, string) (discovery.Watcher, error) {
	return &stubWatcher{}, nil
}

type stubWatcher struct{ done chan struct{} }

func (w *stubWatcher) Next() ([]discovery.Endpoint, error) {
	if w.done == nil {
		w.done = make(chan struct{})
	}
	<-w.done
	return nil, context.Canceled
}

func (w *stubWatcher) Stop() error {
	if w.done != nil {
		close(w.done)
	}
	return nil
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
	rt, closeFn, err := NewTransport(Config{Base: rec})
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
	rt, closeFn, err := NewTransport(Config{Addr: "10.9.8.7:80", Base: rec})
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
	discovery.Register("stub-httpx", stubDiscovery{eps: []discovery.Endpoint{
		{Addr: "10.0.0.1:9000", Healthy: true},
		{Addr: "10.0.0.2:9000", Healthy: true},
	}})

	rec := &recordRT{}
	rt, closeFn, err := NewTransport(Config{
		ServiceName: "user-svc",
		Discovery:   "stub-httpx",
		Base:        rec,
	})
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
	// ServiceName set but discovery backend not registered -> fail fast.
	_, _, err := NewTransport(Config{ServiceName: "x", Discovery: "no-such-backend"})
	assert.Error(t, err).Matches("no backend registered")

	// Unknown balancer strategy -> fail fast.
	discovery.Register("stub-httpx-2", stubDiscovery{eps: []discovery.Endpoint{{Addr: "1.2.3.4:80"}}})
	_, _, err = NewTransport(Config{ServiceName: "x", Discovery: "stub-httpx-2", Balancer: "no-such-lb"})
	assert.Error(t, err).Matches("no strategy registered")
}

func TestNewTransport_ResilienceBreakerFastFails(t *testing.T) {
	rec := &recordRT{status: http.StatusInternalServerError}
	rt, closeFn, err := NewTransport(Config{
		ResilienceDriver: "default",
		ResiliencePolicy: resilience.Policy{ErrorThreshold: 2},
		Base:             rec,
	})
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
