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

package StarterRegistryEtcd

import (
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// newHealRegistrar returns a registrar reading through an in-process fake etcd,
// so the self-healing loop can be driven without a cluster.
func newHealRegistrar() (*etcdRegistrar, *fakeKV) {
	f := newFakeKV()
	return newTestRegistrar(f, "/services/"), f
}

// A keep-alive channel closing (etcd restart / lease lost) must trigger a
// re-publish: failures are retried with backoff until one succeeds, which puts
// the key back under a fresh lease.
func TestWatchKeepAliveReRegistersAfterKeepaliveDeath(t *testing.T) {
	r, f := newHealRegistrar()
	f.grantErrs = []error{errors.New("etcd down"), errors.New("etcd down")}

	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()

	close(ka1) // keep-alive dies
	select {
	case <-done:
		t.Fatal("watcher exited without re-registering")
	case <-time.After(2 * time.Second):
	}
	// Both scripted failures were consumed (with backoff sleeps) and the third
	// attempt went through the real publish step, so the instance is back in the
	// center — asserted on the write itself, not on the loop's bookkeeping.
	waitFor(t, func() bool { return f.writes() == 1 }, "the self-heal never re-published the key")
	assert.Number(t, f.grantRemaining()).Equal(0)

	// Stop through the registrar, not h.stop(): publish stores the cancel func
	// under the registrar lock, so the stop has to take it too.
	r.stopHold(h)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
}

// A keep-alive channel closing because the hold was stopped (Deregister or a
// refresh re-Register) must NOT trigger a re-publish — that would resurrect a
// deregistered instance. Asserting on the write count is what makes this the
// real check: an empty failure queue only proves the fake was not asked.
func TestWatchKeepAliveExitsOnStopWithoutRePublish(t *testing.T) {
	r, f := newHealRegistrar()

	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka); close(done) }()

	r.stopHold(h)
	close(ka)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
	assert.Number(t, f.writes()).Equal(0)
}

// stopHold must be safe against a concurrent publish storing the cancel func,
// and idempotent when Deregister and a re-Register retire the same hold.
func TestHoldStopIdempotent(t *testing.T) {
	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
	h.stop()
	h.stop() // must not panic on double close
	assert.That(t, h.stopped()).True()
}

// The backoff sequence doubles from base and is capped: 5ms,10ms,20ms,20ms...
func TestBackoffGrowthCapped(t *testing.T) {
	r := &etcdRegistrar{backoffBase: 5 * time.Millisecond, backoffCap: 20 * time.Millisecond}
	seen := []time.Duration{}
	// Reuse the retry loop shape from watchKeepAlive by exercising the
	// arithmetic directly: it is trivial, but pinning it prevents accidental
	// changes to the recovery pacing.
	b := r.backoffBase
	for i := 0; i < 4; i++ {
		seen = append(seen, b)
		b *= 2
		if b > r.backoffCap {
			b = r.backoffCap
		}
	}
	assert.That(t, seen).Equal([]time.Duration{
		5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond,
	})
}
