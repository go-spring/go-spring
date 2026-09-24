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

package lock

import (
	"context"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/timeutil"
	"sync"
	"time"
)

// MemoryLocker is a bundled, zero-dependency [Locker] whose scope is a single
// process. It is intended for tests, local development and single-instance
// deployments; for real multi-replica coordination use a Redis/etcd/consul
// backed Locker. Build it with [NewMemoryLocker]: the zero value is not
// usable, and every method fails fast on it rather than panicking on a nil
// map.
type MemoryLocker struct {
	mu    sync.Mutex
	locks map[string]*memoryHold // key -> its CURRENT lease generation
	stop  chan struct{}          // closed by Close; ends every renew loop
	once  sync.Once              // guards Close against a second run
}

// memoryHold is ONE generation of a key's lease. When an expired lease is
// reclaimed, the map gets a brand-new hold — the old holder's memoryLock keeps
// pointing at this superseded one, which is what preserves its token (for the
// ErrNotHeld check) and its own Lost channel. Nothing here is ever recycled.
type memoryHold struct {
	token      string        // fencing token; stable for this generation's whole life
	expireAt   time.Time     // lease deadline, extended by the renew loop
	ttl        time.Duration // lease duration, re-applied on each renewal
	renewEvery time.Duration // renewal pace; non-positive means no renew loop

	// lost closes exactly once, on the END of this generation — voluntary
	// release, expiry, takeover all look the same from the holder's side.
	lost     chan struct{}
	lostOnce sync.Once

	// renew closes to stop this hold's renew loop; Once because drop, unlock
	// and Close may all race to end it.
	renew     chan struct{}
	renewOnce sync.Once
}

func (h *memoryHold) markLost()      { h.lostOnce.Do(func() { close(h.lost) }) }
func (h *memoryHold) stopRenewLoop() { h.renewOnce.Do(func() { close(h.renew) }) }

// NewMemoryLocker returns a ready [MemoryLocker].
func NewMemoryLocker() *MemoryLocker {
	return &MemoryLocker{
		locks: make(map[string]*memoryHold),
		stop:  make(chan struct{}),
	}
}

// TryAcquire implements [Locker].
func (m *MemoryLocker) TryAcquire(_ context.Context, key string, opts ...Option) (Lock, bool, error) {
	o := Resolve(DefaultOptions{}, opts...)
	if m.locks == nil {
		return nil, false, errutil.Explain(nil, "lock: MemoryLocker must be built with NewMemoryLocker")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if h, ok := m.locks[key]; ok {
		if time.Now().Before(h.expireAt) {
			return nil, false, nil // still held by someone
		}
		// Expired: reclaim it, signalling loss to the previous holder.
		m.dropLocked(key, h)
	}

	h := &memoryHold{
		token:      o.Token,
		expireAt:   time.Now().Add(o.TTL),
		ttl:        o.TTL,
		renewEvery: o.RenewInterval,
		lost:       make(chan struct{}),
		renew:      make(chan struct{}),
	}
	m.locks[key] = h
	if h.renewEvery > 0 {
		go m.renewLoop(key, h)
	}
	return &memoryLock{locker: m, key: key, hold: h}, true, nil
}

// Acquire implements [Locker]: it polls TryAcquire until it succeeds or ctx ends.
func (m *MemoryLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	o := Resolve(DefaultOptions{}, opts...)
	for {
		l, ok, err := m.TryAcquire(ctx, key, opts...)
		if err != nil {
			return nil, err
		}
		if ok {
			return l, nil
		}
		if !timeutil.Sleep(ctx, o.RetryInterval) {
			return nil, ctx.Err()
		}
	}
}

// Close implements [Locker]: it stops all renew loops. Held handles remain valid
// but are no longer renewed.
func (m *MemoryLocker) Close() error {
	if m.stop == nil {
		return errutil.Explain(nil, "lock: MemoryLocker must be built with NewMemoryLocker")
	}
	m.once.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		close(m.stop)
	})
	return nil
}

// renewLoop extends the lease while the hold is active, at the hold's own
// renewEvery pace and by its own ttl.
func (m *MemoryLocker) renewLoop(key string, h *memoryHold) {
	ticker := time.NewTicker(h.renewEvery)
	defer ticker.Stop()
	for {
		select {
		case <-h.renew:
			return
		case <-m.stop:
			return
		case <-ticker.C:
			m.mu.Lock()
			cur, ok := m.locks[key]
			if !ok || cur != h {
				// The key is gone or holds a NEWER generation: this lease was
				// released, dropped or taken over while we waited — stop renewing.
				m.mu.Unlock()
				return
			}
			cur.expireAt = time.Now().Add(h.ttl)
			m.mu.Unlock()
		}
	}
}

// dropLocked removes a hold from the map and ends its generation: the renew
// loop is told to stop and the old holder's Lost channel is closed. Caller
// holds m.mu.
func (m *MemoryLocker) dropLocked(key string, h *memoryHold) {
	delete(m.locks, key)
	h.stopRenewLoop()
	h.markLost()
}

// unlock releases key if still owned by token. It does not signal loss (voluntary
// release); the handle closes its own lost channel so [Lock.Lost] also fires.
func (m *MemoryLocker) unlock(key, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	h, ok := m.locks[key]
	if !ok {
		return nil // already released or expired
	}
	if h.token != token {
		return ErrNotHeld // taken over by another owner
	}
	delete(m.locks, key)
	h.stopRenewLoop()
	return nil
}

// memoryLock is a handle returned by MemoryLocker.
type memoryLock struct {
	locker *MemoryLocker
	key    string
	hold   *memoryHold
	once   sync.Once
}

func (l *memoryLock) Key() string           { return l.key }
func (l *memoryLock) Token() string         { return l.hold.token }
func (l *memoryLock) Lost() <-chan struct{} { return l.hold.lost }

func (l *memoryLock) Unlock(_ context.Context) error {
	// Idempotent by design: only the first call releases. Later calls skip the
	// closure, so err stays nil — a repeated Unlock returns nil even when the
	// first returned ErrNotHeld, exactly the contract [Lock.Unlock] promises.
	var err error
	l.once.Do(func() {
		err = l.locker.unlock(l.key, l.hold.token)
		l.hold.markLost()
	})
	return err
}
