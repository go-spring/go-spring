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

package StarterDiscoveryConsul

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestServiceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, serviceID(discovery.Instance{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, serviceID(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestRegister_BadAddr(t *testing.T) {
	// api.NewClient does not dial, so this needs no live agent: the addr is
	// validated before any Consul call, so a malformed addr fails fast.
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	assert.Error(t, err).Nil()
	r := &consulRegistry{client: client, ttl: time.Second, heartbeats: map[string]chan struct{}{}}

	err = r.Register(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "no-port"})
	assert.Error(t, err).Matches("must be host:port")

	err = r.Register(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "host:abc"})
	assert.Error(t, err).Matches("non-numeric port")
}

func TestNormalizeWeight(t *testing.T) {
	// A misconfigured negative weight clamps to 1.
	assert.That(t, normalizeWeight(-5)).Equal(1)
	// 0 is the drain signal and passes through untouched — both Register and
	// UpdateWeight feed their weight through this helper, so a configured 0
	// reaches buildRegistration and is advertised as a zero Passing weight
	// (see TestBuildRegistration_AdvertisesDrainWeight).
	assert.That(t, normalizeWeight(0)).Equal(0)
	// An explicit positive weight passes through unchanged.
	assert.That(t, normalizeWeight(100)).Equal(100)
}

func TestUpdateWeight_Unregistered(t *testing.T) {
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:8500"})
	assert.Error(t, err).Nil()
	r := &consulRegistry{client: client, heartbeats: map[string]chan struct{}{}, regs: map[string]discovery.Instance{}}

	err = r.UpdateWeight(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"}, 5)
	assert.Error(t, err).Matches("unregistered instance")
}

func TestBuildRegistration_AdvertisesDrainWeight(t *testing.T) {
	// A drain weight of 0 is advertised as a zero passing weight — Consul
	// routes no traffic to it — rather than being normalized away.
	r := &consulRegistry{heartbeats: map[string]chan struct{}{}, regs: map[string]discovery.Instance{}}
	asr := r.buildRegistration(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0})
	assert.That(t, asr.Weights.Passing).Equal(0)
}

func TestReRegister(t *testing.T) {
	// An unknown id is a no-op (the instance was deregistered; the heartbeat's
	// recovery path must not resurrect it).
	r := &consulRegistry{heartbeats: map[string]chan struct{}{}, regs: map[string]discovery.Instance{}}
	assert.Error(t, r.reRegister("orders-1.2.3.4:80")).Nil()

	// A known id attempts the upsert against the (here unreachable) agent: a
	// connection error proves the recovery path actually re-registers rather
	// than silently doing nothing, and that the last advertised value — the
	// drained weight — is what gets re-registered.
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:1"})
	assert.Error(t, err).Nil()
	r = &consulRegistry{client: client, heartbeats: map[string]chan struct{}{}, regs: map[string]discovery.Instance{}}
	drained := discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0}
	r.regs["orders-1.2.3.4:80"] = drained
	asr := r.buildRegistration(drained)
	assert.That(t, asr.Weights.Passing).Equal(0)
}

// closedPortClient returns a Consul client whose agent address has nothing
// listening: api.NewClient does not dial, so every call fails at the transport.
func closedPortClient(t *testing.T) *api.Client {
	t.Helper()
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:1"})
	assert.Error(t, err).Nil()
	return client
}

// reachableClient returns a Consul client pointed at a fake agent that answers
// any request with 200, so the write paths exercise their success branches.
func reachableClient(t *testing.T) *api.Client {
	t.Helper()
	srv := httptest.NewServer(new(fakeConsul))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: srv.URL})
	assert.Error(t, err).Nil()
	return client
}

// A failed initial publish must be counted and must leave the instance reported
// as unpublished.
func TestRegisterFailureIsReported(t *testing.T) {
	r := &consulRegistry{
		obs:        newTestObserver(),
		client:     closedPortClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]discovery.Instance{},
	}

	err := r.Register(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	assert.Error(t, err).NotNil()

	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders",
		"reason": discovery.ReasonInitial, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": "orders",
	})).Zero()
}

// The TTL heartbeat's self-healing escalation (reRegister) never passes through
// the Registry interface, so this asserts it is reported anyway — with
// reason=self_heal, and with the gauge following each attempt's outcome.
func TestSelfHealIsReported(t *testing.T) {
	in := discovery.Instance{ServiceName: "payments", Addr: "1.2.3.4:80", Weight: 1}
	id := serviceID(in)
	r := &consulRegistry{
		obs:        newTestObserver(),
		client:     closedPortClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]discovery.Instance{id: in},
	}
	gauge := map[string]string{"system": obsSystem, "service": "payments"}

	// The agent is unreachable: the escalation fails and the instance is no
	// longer discoverable, which is the state worth alerting on.
	assert.Error(t, r.reRegister(id)).NotNil()
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "payments",
		"reason": discovery.ReasonSelfHeal, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", gauge)).Zero()

	// The agent comes back: the same escalation succeeds and the gauge clears.
	r.client = reachableClient(t)
	assert.Error(t, r.reRegister(id)).Nil()
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "payments",
		"reason": discovery.ReasonSelfHeal, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", gauge)).Equal(int64(1))
}

// A repeat Deregister is the normal shutdown path, not a failure: the discovery
// core deregisters from both PreStop and the Stop fallback. Consul answering
// 404 for an id it no longer holds is that no-op, and it must be reported as
// success so a clean shutdown does not look like a wave of deregister failures.
func TestDeregisterRepeatIsANoOp(t *testing.T) {
	in := discovery.Instance{ServiceName: "orders-repeat", Addr: "1.2.3.4:80", Weight: 1}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`Unknown service ID "orders-repeat-1.2.3.4:80"`))
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: srv.URL})
	assert.Error(t, err).Nil()
	r := &consulRegistry{
		obs:        newTestObserver(),
		client:     client,
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]discovery.Instance{},
	}

	assert.Error(t, r.Deregister(context.Background(), in)).Nil()
	assert.Number(t, histCount(t, "discovery.operation.duration", map[string]string{
		"system": obsSystem, "operation": "deregister", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))
}

// Deregistration and weight re-advertisement are first-class operations on the
// duration metric — the drain path in particular, which the discovery core does
// not log.
func TestDeregisterAndWeightChangeAreReported(t *testing.T) {
	in := discovery.Instance{ServiceName: "orders-drain", Addr: "1.2.3.4:80", Weight: 1}
	id := serviceID(in)
	r := &consulRegistry{
		obs:        newTestObserver(),
		client:     reachableClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]discovery.Instance{id: in},
	}
	ctx := context.Background()

	assert.Error(t, r.UpdateWeight(ctx, in, 0)).Nil()
	assert.Number(t, histCount(t, "discovery.operation.duration", map[string]string{
		"system": obsSystem, "operation": "update_weight", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))

	assert.Error(t, r.Deregister(ctx, in)).Nil()
	assert.Number(t, histCount(t, "discovery.operation.duration", map[string]string{
		"system": obsSystem, "operation": "deregister", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero("a deregistered instance is no longer published")
}
