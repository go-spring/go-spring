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
	"sync"
	"time"
)

// MemoryLocker is a bundled, zero-dependency [Locker] whose scope is a single
// process. It is intended for tests, local development and single-instance
// deployments; for real multi-replica coordination use a Redis/etcd/consul
// backed Locker. Prefer [NewMemoryLocker] so its internal maps are initialized.
type MemoryLocker struct {
	mu    sync.Mutex
	locks map[string]*memoryHold
	stop  chan struct{}
	once  sync.Once
}

type memoryHold struct {
	token     string
	expireAt  time.Time
	lost      chan struct{}
	lostOnce  sync.Once
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

func (m *MemoryLocker) ensure() {
	if m.locks == nil {
		m.locks = make(map[string]*memoryHold)
	}
	if m.stop == nil {
		m.stop = make(chan struct{})
	}
}

// TryAcquire implements [Locker].
func (m *MemoryLocker) TryAcquire(_ context.Context, key string, opts ...Option) (Lock, bool, error) {
	o := Apply(opts...)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensure()

	if h, ok := m.locks[key]; ok {
		if time.Now().Before(h.expireAt) {
			return nil, false, nil // still held by someone
		}
		// Expired: reclaim it, signalling loss to the previous holder.
		m.dropLocked(key, h)
	}

	h := &memoryHold{
		token:    o.Token,
		expireAt: time.Now().Add(o.TTL),
		lost:     make(chan struct{}),
		renew:    make(chan struct{}),
	}
	m.locks[key] = h
	if o.RenewInterval > 0 {
		go m.renewLoop(key, h, o.TTL, o.RenewInterval)
	}
	return &memoryLock{locker: m, key: key, hold: h}, true, nil
}

// Acquire implements [Locker]: it polls TryAcquire until it succeeds or ctx ends.
func (m *MemoryLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	o := Apply(opts...)
	for {
		l, ok, err := m.TryAcquire(ctx, key, opts...)
		if err != nil {
			return nil, err
		}
		if ok {
			return l, nil
		}
		t := time.NewTimer(o.RetryInterval)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
}

// Close implements [Locker]: it stops all renew loops. Held handles remain valid
// but are no longer renewed.
func (m *MemoryLocker) Close() error {
	m.once.Do(func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.ensure()
		close(m.stop)
	})
	return nil
}

// renewLoop extends the lease while the hold is active.
func (m *MemoryLocker) renewLoop(key string, h *memoryHold, ttl, every time.Duration) {
	ticker := time.NewTicker(every)
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
				m.mu.Unlock()
				return
			}
			cur.expireAt = time.Now().Add(ttl)
			m.mu.Unlock()
		}
	}
}

// dropLocked removes a hold and signals its loss. Caller holds m.mu.
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
	var err error
	l.once.Do(func() {
		err = l.locker.unlock(l.key, l.hold.token)
		l.hold.markLost()
	})
	return err
}
