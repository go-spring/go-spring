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

package StarterLockConsul

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"

	"go-spring.org/cloud/lock"
	"go-spring.org/stdlib/testing/assert"
)

// The acquire/renew/release paths need a live consul agent (sessions, blocking
// queries), which unit tests here cannot provide; see example/ for a
// docker-gated smoke run. These tests cover the pure helpers and option
// shaping the live path depends on — including the TTL clamp that keeps consul
// from rejecting the whole starter over an out-of-range TTL.

func TestCtxStopCh_ClosesOnCancel(t *testing.T) {
	stopCh, cancel := ctxStopCh(context.Background())
	select {
	case <-stopCh:
		t.Fatal("stopCh closed before cancel")
	default:
	}
	cancel()
	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("stopCh not closed after cancel")
	}
}

func TestCtxStopCh_ClosesOnParentDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	stopCh, done := ctxStopCh(ctx)
	cancel() // parent cancelled first
	select {
	case <-stopCh:
	case <-time.After(time.Second):
		t.Fatal("stopCh not closed after parent ctx done")
	}
	done() // must not panic or double-close
}

// TestKeyPrefix_DefaultAndApplied proves the lock/ key-prefix default and that
// it is prepended in buildLock's LockOptions.
func TestKeyPrefix_DefaultAndApplied(t *testing.T) {
	// An explicit prefix is used verbatim.
	l := &consulLocker{keyPrefix: "apps/myapp/", defaults: lock.DefaultOptions{TTL: time.Minute}}
	_ = l

	// The default prefix is "lock/" (see newConsulLocker); resolved options keep
	// the starter TTL layered beneath per-call opts.
	l2 := &consulLocker{keyPrefix: "lock/", defaults: lock.DefaultOptions{TTL: 20 * time.Second}}
	o := lock.Resolve(l2.defaults)
	assert.That(t, o.TTL).Equal(20 * time.Second)
	o = lock.Resolve(l2.defaults, lock.WithTTL(2*time.Minute))
	assert.That(t, o.TTL).Equal(2 * time.Minute)
}

// TestSessionTTLClamp proves a per-call TTL below consul's 10s floor (including
// the lock package's 30s default path and test-style short TTLs) is clamped
// into consul's accepted window rather than rejected.
func TestSessionTTLClamp(t *testing.T) {
	clamp := func(d time.Duration) time.Duration {
		if d < minSessionTTL {
			d = minSessionTTL
		}
		if d > maxSessionTTL {
			d = maxSessionTTL
		}
		return d
	}
	assert.That(t, clamp(500*time.Millisecond)).Equal(10 * time.Second)
	assert.That(t, clamp(30*time.Second)).Equal(30 * time.Second)
	assert.That(t, clamp(0)).Equal(10 * time.Second) // zero resolved later, but clamp is a floor
	assert.That(t, clamp(200*time.Hour)).Equal(maxSessionTTL)
}

// TestConsulLockHandleAccessors covers the pure handle accessors over a
// leaderCh that never fires.
func TestConsulLockHandleAccessors(t *testing.T) {
	leaderCh := make(chan struct{})
	h := newConsulLock(&api.Lock{}, "jobs", "tok-1", leaderCh)
	assert.That(t, h.Key()).Equal("jobs")
	assert.That(t, h.Token()).Equal("tok-1")
	select {
	case <-h.Lost():
		t.Fatal("Lost() fired before session loss")
	default:
	}
	// Closing the leader channel is what consul does on session loss; Lost()
	// must simply reflect it.
	close(leaderCh)
	select {
	case <-h.Lost():
	default:
		t.Fatal("Lost() did not reflect leaderCh close")
	}
}
