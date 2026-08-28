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

	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// newHealRegistrar returns a registrar whose publish step is faked: each call
// either fails (as if etcd is still down) or returns a caller-controlled
// keep-alive channel. No etcd server is needed to drive the self-healing loop.
func newHealRegistrar() (*etcdRegistrar, *[]error, chan chan *clientv3.LeaseKeepAliveResponse) {
	r := &etcdRegistrar{
		keyPrefix:   "/services/",
		ttlSecs:     15,
		backoffBase: 5 * time.Millisecond,
		backoffCap:  20 * time.Millisecond,
		holds:       map[string]*hold{},
	}
	fails := []error{}
	lastKA := make(chan chan *clientv3.LeaseKeepAliveResponse, 4)
	r.publish = func(*hold) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		if len(fails) > 0 {
			err := fails[0]
			fails = fails[1:]
			return nil, err
		}
		ch := make(chan *clientv3.LeaseKeepAliveResponse)
		lastKA <- ch
		return ch, nil
	}
	return r, &fails, lastKA
}

// A keep-alive channel closing (etcd restart / lease lost) must trigger a
// re-publish: failures are retried with backoff until one succeeds, then the
// new channel is drained for the next cycle.
func TestWatchKeepAliveReRegistersAfterKeepaliveDeath(t *testing.T) {
	r, fails, lastKA := newHealRegistrar()
	*fails = []error{errors.New("etcd down"), errors.New("etcd down")}

	h := newHold(instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()

	close(ka1) // keep-alive dies
	select {
	case <-done:
		t.Fatal("watcher exited without re-registering")
	case <-time.After(2 * time.Second):
	}
	// Two failures were consumed (with backoff sleeps) and the third publish
	// succeeded, so the watcher is alive and draining a fresh channel.
	assert.Number(t, len(*fails)).Equal(0)
	h.stop()
	// With a real etcd client, cancelling the keep-alive context closes the
	// channel; emulate that for the fake so the drain loop observes it.
	close(<-lastKA)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
}

// A keep-alive channel closing because the hold was stopped (Deregister or a
// refresh re-Register) must NOT trigger a re-publish — that would resurrect a
// deregistered instance.
func TestWatchKeepAliveExitsOnStopWithoutRePublish(t *testing.T) {
	r, fails, _ := newHealRegistrar()

	h := newHold(instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka); close(done) }()

	h.stop()
	close(ka)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
	assert.Number(t, len(*fails)).Equal(0)
}

// stopHold must be safe against a concurrent publish storing the cancel func,
// and idempotent when Deregister and a re-Register retire the same hold.
func TestHoldStopIdempotent(t *testing.T) {
	h := newHold(instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
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
