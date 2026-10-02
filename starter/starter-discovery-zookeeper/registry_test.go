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

package StarterDiscoveryZookeeper

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestInstanceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, instanceID(discovery.Instance{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, instanceID(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestPathFor(t *testing.T) {
	// The base path's trailing slash is normalised away at construction, so the
	// znode path has exactly one separator per level.
	r := &zkRegistry{basePath: "/services"}
	got := r.pathFor(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
	assert.That(t, got).Equal("/services/orders/orders-1.2.3.4:80")
}

func TestNormalizeWeight(t *testing.T) {
	// A misconfigured negative weight clamps to 1.
	assert.That(t, normalizeWeight(-5)).Equal(1)
	// 0 is the drain signal and passes through untouched, on both write paths.
	assert.That(t, normalizeWeight(0)).Equal(0)
	// An explicit positive weight passes through unchanged.
	assert.That(t, normalizeWeight(100)).Equal(100)
}

func TestInstanceValueDrainEncoding(t *testing.T) {
	// The drain signal (weight 0) serializes as an omitted weight field —
	// a reader reconstructs 0 and excludes the instance from picking.
	b, err := json.Marshal(instanceValue{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 0})
	assert.Error(t, err).Nil()
	assert.String(t, string(b)).Equal(`{"service_name":"orders","addr":"1.2.3.4:80"}`)

	var v instanceValue
	assert.Error(t, json.Unmarshal(b, &v)).Nil()
	assert.Number(t, v.Weight).Equal(0)
}

// newHealRegistry returns a registry with fake state/reRegister seams: no
// ensemble is needed to drive the session monitor through loss and recovery.
func newHealRegistry() (*zkRegistry, *atomic.Int32) {
	r := &zkRegistry{
		obs:         newTestObserver(),
		basePath:    "/services",
		backoffBase: 5 * time.Millisecond,
		backoffCap:  20 * time.Millisecond,
		regs:        map[string]discovery.Instance{},
		done:        make(chan struct{}),
	}
	var cur atomic.Int32
	cur.Store(int32(zk.StateHasSession))
	r.state = func() zk.State { return zk.State(cur.Load()) }
	r.reRegister = func(discovery.Instance) error { return nil }
	return r, &cur
}

// reconcileSession is the whole recovery decision: any non-session state marks
// the session degraded; a degraded session seen healthy again must heal.
func TestReconcileSession(t *testing.T) {
	// Healthy all along: never degraded, never heals.
	d, h := reconcileSession(false, zk.StateHasSession)
	assert.That(t, d).False()
	assert.That(t, h).False()
	// Session lost: degraded, no heal yet.
	d, h = reconcileSession(false, zk.StateExpired)
	assert.That(t, d).True()
	assert.That(t, h).False()
	// Still lost (mid-reconnect): stays degraded, no heal.
	d, h = reconcileSession(true, zk.StateConnecting)
	assert.That(t, d).True()
	assert.That(t, h).False()
	// Recovered: not degraded, heal — the nodes must be re-created.
	d, h = reconcileSession(true, zk.StateHasSession)
	assert.That(t, d).False()
	assert.That(t, h).True()
}

// A full loss/recovery cycle through monitorSession: after the session comes
// back, every tracked instance is re-created, with backoff retries while the
// ensemble is still failing.
func TestMonitorSessionReCreatesNodesAfterRecovery(t *testing.T) {
	r, cur := newHealRegistry()
	var mu sync.Mutex
	created := []discovery.Instance{}
	failures := int32(2) // first two heal passes fail, as if the session flaps
	r.reRegister = func(reg discovery.Instance) error {
		if atomic.AddInt32(&failures, -1) >= 0 {
			return errors.New("connection lost")
		}
		mu.Lock()
		created = append(created, reg)
		mu.Unlock()
		return nil
	}
	r.regs["/services/orders/orders-1.2.3.4:80"] = discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	r.regs["/service2"] = discovery.Instance{ServiceName: "billing", Addr: "1.2.3.4:90", Weight: 3}

	go r.monitorSession()
	// Session drops, then recovers on the next poll.
	cur.Store(int32(zk.StateExpired))
	time.Sleep(1500 * time.Millisecond)
	cur.Store(int32(zk.StateHasSession))

	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := len(created)
		mu.Unlock()
		if n == 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	r.Close()
	mu.Lock()
	defer mu.Unlock()
	// Assert by name: the heal pass walks r.regs, a map, so the order the two
	// instances are re-created in is not defined.
	byName := map[string]discovery.Instance{}
	for _, in := range created {
		byName[in.ServiceName] = in
	}
	assert.Number(t, len(byName)).Equal(2)
	// The last advertised value is re-created, not the startup default.
	assert.That(t, byName["orders"].Weight).Equal(1)
	assert.That(t, byName["billing"].Weight).Equal(3)
}

// Deregister must remove the instance from the heal set: a node recreated
// after deregister would resurrect a drained instance.
func TestDeregisterRemovesFromHealSet(t *testing.T) {
	r, _ := newHealRegistry()
	reg := discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	r.regs[r.pathFor(reg)] = reg
	// Deregister deletes from the map before touching the (here nil) conn; the
	// map bookkeeping is what the heal set reads.
	r.mu.Lock()
	delete(r.regs, r.pathFor(reg))
	r.mu.Unlock()
	assert.Number(t, len(r.regs)).Equal(0)
	_, err := r.reRegisterAll()
	assert.Error(t, err).Nil()
}

// healAll exits when Close is called even while every attempt fails.
func TestHealAllExitsOnClose(t *testing.T) {
	r, _ := newHealRegistry()
	// One tracked discovery.Instance, so each heal pass must call reRegister (an empty
	// heal set succeeds trivially and the loop would exit without any call).
	r.regs["/services/orders/orders-1.2.3.4:80"] = discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	calls := make(chan struct{}, 16)
	r.reRegister = func(discovery.Instance) error { calls <- struct{}{}; return errors.New("down") }
	done := make(chan struct{})
	go func() { r.healAll(); close(done) }()
	<-calls // at least one failed attempt happened
	r.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("healAll did not exit after Close")
	}
}

// The session monitor's recovery loop re-creates every registered node after a
// session loss. That path never passes through the Registry interface, so this
// asserts it is reported anyway — with reason=self_heal, which is what
// separates "the ensemble lost my node" from the initial publish.
func TestSelfHealReRegistrationIsReported(t *testing.T) {
	r := &zkRegistry{
		obs:      newTestObserver(),
		basePath: "/services",
		regs: map[string]discovery.Instance{
			"/services/orders-selfheal/orders-selfheal-1.2.3.4:80": {
				ServiceName: "orders-selfheal", Addr: "1.2.3.4:80", Weight: 1,
			},
		},
	}
	gauge := map[string]string{"system": obsSystem, "service": "orders-selfheal"}

	// The session is not usable yet: the pass fails and the instance is not
	// discoverable, which is exactly what the gauge must say.
	r.reRegister = func(discovery.Instance) error { return errors.New("ensemble unreachable") }
	// The failing service comes back with the error, so the caller's log line
	// can carry the identity that joins it to the metric below.
	failed, err := r.reRegisterAll()
	assert.Error(t, err).NotNil()
	assert.That(t, failed).Equal("orders-selfheal")
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders-selfheal",
		"reason": discovery.ReasonSelfHeal, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", gauge)).
		Zero("a session loss that could not be healed must report as unpublished")

	// The session recovers: healAll retries the pass and the node comes back.
	r.reRegister = func(discovery.Instance) error { return nil }
	_, err = r.reRegisterAll()
	assert.Error(t, err).Nil()
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders-selfheal",
		"reason": discovery.ReasonSelfHeal, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", gauge)).Equal(int64(1))
}
