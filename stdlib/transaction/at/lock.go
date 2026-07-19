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

package at

import (
	"context"
	"errors"
	"sync"
)

// ErrLockConflict is returned by [GlobalLock.Acquire] when a row it wants is
// already locked by a different global transaction. A resource interceptor
// surfaces it as the statement's error, so the writing transaction fails and
// rolls back rather than staging a change that conflicts with an in-flight one.
var ErrLockConflict = errors.New("at: global row lock conflict")

// MemoryGlobalLock is a zero-dependency, concurrency-safe in-process [GlobalLock].
// It is the AT equivalent of [go-spring.org/stdlib/lock.MemoryLocker]: it holds
// row locks in a map keyed by [LockKey], owning each by XID. It provides real
// write-write isolation for a single process (several goroutines driving
// concurrent global transactions); a multi-process deployment needs a shared
// backend instead. The zero value is ready to use.
type MemoryGlobalLock struct {
	mu    sync.Mutex
	owner map[LockKey]string // row key -> owning xid
}

var _ GlobalLock = (*MemoryGlobalLock)(nil)

// Acquire locks every key for xid, all-or-nothing. If any key is held by another
// xid it returns [ErrLockConflict] and acquires none. Re-locking a key already
// held by xid is a no-op.
func (m *MemoryGlobalLock) Acquire(_ context.Context, xid string, keys []LockKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// First pass: reject if any key is held by a different transaction, so we never
	// leave a partial acquisition behind on conflict.
	for _, k := range keys {
		if owner, held := m.owner[k]; held && owner != xid {
			return ErrLockConflict
		}
	}
	if m.owner == nil {
		m.owner = make(map[LockKey]string)
	}
	for _, k := range keys {
		m.owner[k] = xid
	}
	return nil
}

// Release frees every key held by xid. Releasing when nothing is held is a no-op,
// so it is idempotent.
func (m *MemoryGlobalLock) Release(_ context.Context, xid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, owner := range m.owner {
		if owner == xid {
			delete(m.owner, k)
		}
	}
	return nil
}
