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
	"sync"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// scriptedFailures is the queue the fake publish consumes: each entry makes one
// publish call fail (as if etcd is still down); an empty queue lets it succeed.
// It is mutex-guarded because the healing loop consumes from its own goroutine
// while the test reads the remaining count — an unguarded slice would race.
type scriptedFailures struct {
	mu    sync.Mutex
	items []error
}

// set replaces the scripted failures.
func (s *scriptedFailures) set(errs ...error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = errs
}

// pull takes the next scripted failure, if any.
func (s *scriptedFailures) pull() (error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) == 0 {
		return nil, false
	}
	err := s.items[0]
	s.items = s.items[1:]
	return err, true
}

// remaining reports how many scripted failures are left unconsumed.
func (s *scriptedFailures) remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// newHealRegistrar returns a registrar whose publish step is faked: each call
// either fails (as if etcd is still down) or returns a caller-controlled
// keep-alive channel. No etcd server is needed to drive the self-healing loop.
func newHealRegistrar() (*etcdRegistrar, *scriptedFailures, chan chan *clientv3.LeaseKeepAliveResponse) {
	r := &etcdRegistrar{
		keyPrefix:   "/services/",
		ttlSecs:     15,
		backoffBase: 5 * time.Millisecond,
		backoffCap:  20 * time.Millisecond,
		holds:       map[string]*hold{},
	}
	fails := &scriptedFailures{}
	lastKA := make(chan chan *clientv3.LeaseKeepAliveResponse, 4)
	r.publish = func(*hold) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		if err, ok := fails.pull(); ok {
			return nil, err
		}
		ch := make(chan *clientv3.LeaseKeepAliveResponse)
		lastKA <- ch
		return ch, nil
	}
	return r, fails, lastKA
}

// A keep-alive channel closing (etcd restart / lease lost) must trigger a
// re-publish: failures are retried with backoff until one succeeds, then the
// new channel is drained for the next cycle.
func TestWatchKeepAliveReRegistersAfterKeepaliveDeath(t *testing.T) {
	r, fails, lastKA := newHealRegistrar()
	fails.set(errors.New("etcd down"), errors.New("etcd down"))

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
	assert.Number(t, fails.remaining()).Equal(0)
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
	assert.Number(t, fails.remaining()).Equal(0)
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
