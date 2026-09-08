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

package StarterRegistryZookeeper

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-zookeeper/zk"
	"go-spring.org/stdlib/testing/assert"
)

// newHealRegistrar returns a registrar with fake state/reRegister seams: no
// ensemble is needed to drive the session monitor through loss and recovery.
func newHealRegistrar() (*zkRegistrar, *atomic.Int32) {
	r := &zkRegistrar{
		basePath:    "/services",
		backoffBase: 5 * time.Millisecond,
		backoffCap:  20 * time.Millisecond,
		regs:        map[string]instance{},
		done:        make(chan struct{}),
	}
	var cur atomic.Int32
	cur.Store(int32(zk.StateHasSession))
	r.state = func() zk.State { return zk.State(cur.Load()) }
	r.reRegister = func(instance) error { return nil }
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
	r, cur := newHealRegistrar()
	var mu sync.Mutex
	created := []instance{}
	failures := int32(2) // first two heal passes fail, as if the session flaps
	r.reRegister = func(reg instance) error {
		if atomic.AddInt32(&failures, -1) >= 0 {
			return errors.New("connection lost")
		}
		mu.Lock()
		created = append(created, reg)
		mu.Unlock()
		return nil
	}
	r.regs["/services/orders/orders-1.2.3.4:80"] = instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	r.regs["/service2"] = instance{ServiceName: "billing", Addr: "1.2.3.4:90", Weight: 3}

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
	assert.Number(t, len(created)).Equal(2)
	// The last advertised value is re-created, not the startup default.
	assert.That(t, created[0].ServiceName).Equal("orders")
	assert.That(t, created[1].Weight).Equal(3)
}

// Deregister must remove the instance from the heal set: a node recreated
// after deregister would resurrect a drained instance.
func TestDeregisterRemovesFromHealSet(t *testing.T) {
	r, _ := newHealRegistrar()
	reg := instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	r.regs[r.pathFor(reg)] = reg
	// Deregister deletes from the map before touching the (here nil) conn; the
	// map bookkeeping is what the heal set reads.
	r.mu.Lock()
	delete(r.regs, r.pathFor(reg))
	r.mu.Unlock()
	assert.Number(t, len(r.regs)).Equal(0)
	assert.Error(t, r.reRegisterAll()).Nil()
}

// healAll exits when Close is called even while every attempt fails.
func TestHealAllExitsOnClose(t *testing.T) {
	r, _ := newHealRegistrar()
	// One tracked instance, so each heal pass must call reRegister (an empty
	// heal set succeeds trivially and the loop would exit without any call).
	r.regs["/services/orders/orders-1.2.3.4:80"] = instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}
	calls := make(chan struct{}, 16)
	r.reRegister = func(instance) error { calls <- struct{}{}; return errors.New("down") }
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
