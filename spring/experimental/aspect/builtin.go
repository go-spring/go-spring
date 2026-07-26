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

package aspect

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

// Recover returns an interceptor that turns a panic raised anywhere further down
// the chain (including the target) into an error, so a single misbehaving
// operation cannot crash the process. The recovered value is wrapped with %v; if
// it already is an error it is preserved via errors.Is/As through %w.
func Recover() Interceptor {
	return InterceptorFunc(func(jp *Joinpoint) (result any, err error) {
		defer func() {
			if r := recover(); r != nil {
				if e, ok := r.(error); ok {
					err = fmt.Errorf("aspect: recovered panic in %q: %w", jp.Method, e)
				} else {
					err = fmt.Errorf("aspect: recovered panic in %q: %v", jp.Method, r)
				}
				result = nil
			}
		}()
		return jp.Proceed(jp.Context)
	})
}

// Timing returns an interceptor that measures how long the remaining chain takes
// and reports it through report, together with the method name and final error.
// It is the seam for method-level metrics and audit logging (the埋点/审计
// equivalent). report is called even when the operation fails so failures are
// observable; it must be safe for concurrent use.
func Timing(report func(method string, d time.Duration, err error)) Interceptor {
	return InterceptorFunc(func(jp *Joinpoint) (any, error) {
		start := time.Now()
		v, err := jp.Proceed(jp.Context)
		if report != nil {
			report(jp.Method, time.Since(start), err)
		}
		return v, err
	})
}

// KeyFunc derives a cache key from a joinpoint. Returning an empty string tells
// [Cache] to skip caching for this invocation (neither read nor write).
type KeyFunc func(jp *Joinpoint) string

// Store is the backend a [Cache] interceptor reads and writes. Implementations
// must be safe for concurrent use. The bundled [MemoryStore] is a zero-dependency
// in-process implementation; a starter may adapt Redis or another cache to this
// interface without changing the interceptor.
type Store interface {
	// Get returns the cached value for key and whether it was present (and not
	// expired).
	Get(key string) (any, bool)
	// Set stores val under key for the given ttl. A non-positive ttl means the
	// entry does not expire.
	Set(key string, val any, ttl time.Duration)
}

// Cache returns an interceptor that provides the @Cacheable equivalent: on a hit
// it returns the stored value and skips the rest of the chain entirely; on a miss
// it proceeds and, when the operation succeeds, stores the result under ttl. When
// key returns an empty string caching is bypassed for that invocation. A nil
// store makes the interceptor a transparent pass-through.
func Cache(store Store, key KeyFunc, ttl time.Duration) Interceptor {
	return InterceptorFunc(func(jp *Joinpoint) (any, error) {
		if store == nil || key == nil {
			return jp.Proceed(jp.Context)
		}
		k := key(jp)
		if k == "" {
			return jp.Proceed(jp.Context)
		}
		if v, ok := store.Get(k); ok {
			jp.Result = v
			return v, nil
		}
		v, err := jp.Proceed(jp.Context)
		if err == nil {
			store.Set(k, v, ttl)
		}
		return v, err
	})
}

// MemoryStore is a zero-dependency, concurrency-safe in-process [Store] with
// per-entry expiry. It is intended for tests and single-instance caching; use a
// shared cache (Redis, memcached) behind the [Store] interface for multi-replica
// deployments. The zero value is ready to use.
type MemoryStore struct {
	mu      sync.RWMutex
	entries map[string]memoryEntry
}

type memoryEntry struct {
	val      any
	expireAt time.Time // zero means no expiry
}

// Get implements [Store]. An expired entry is treated as absent and lazily
// dropped.
func (m *MemoryStore) Get(key string) (any, bool) {
	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.expireAt.IsZero() && time.Now().After(e.expireAt) {
		m.mu.Lock()
		// Re-check under the write lock: a concurrent Set may have refreshed it.
		if cur, ok := m.entries[key]; ok && cur.expireAt.Equal(e.expireAt) {
			delete(m.entries, key)
		}
		m.mu.Unlock()
		return nil, false
	}
	return e.val, true
}

// Set implements [Store]. A non-positive ttl stores the entry without expiry.
func (m *MemoryStore) Set(key string, val any, ttl time.Duration) {
	e := memoryEntry{val: val}
	if ttl > 0 {
		e.expireAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	if m.entries == nil {
		m.entries = make(map[string]memoryEntry)
	}
	m.entries[key] = e
	m.mu.Unlock()
}

// TxManager abstracts a transactional resource so [Transactional] can bracket an
// operation without depending on any concrete database library. Begin opens a
// transaction and returns a context carrying it (so downstream code discovers the
// transaction through the context) together with an opaque handle; Commit and
// Rollback finalize that handle. A gorm/sql/etc. adapter implements this
// interface in a starter or in application code.
type TxManager interface {
	Begin(ctx context.Context) (context.Context, any, error)
	Commit(tx any) error
	Rollback(tx any) error
}

// Transactional returns an interceptor that provides the declarative-transaction
// equivalent (@Transactional): it begins a transaction, proceeds with the
// tx-carrying context so the business code runs inside it, then commits on
// success or rolls back on error. A panic further down the chain triggers a
// rollback and is re-raised so an outer [Recover] can translate it. A nil
// TxManager makes the interceptor a transparent pass-through.
func Transactional(tm TxManager) Interceptor {
	return InterceptorFunc(func(jp *Joinpoint) (result any, err error) {
		if tm == nil {
			return jp.Proceed(jp.Context)
		}
		ctx, tx, err := tm.Begin(jp.Context)
		if err != nil {
			return nil, fmt.Errorf("aspect: begin transaction for %q: %w", jp.Method, err)
		}
		committed := false
		defer func() {
			if committed {
				return
			}
			// Rollback on error or on a propagating panic. Preserve the original
			// error; surface a rollback failure only when there was none.
			if rbErr := tm.Rollback(tx); rbErr != nil && err == nil {
				err = fmt.Errorf("aspect: rollback transaction for %q: %w", jp.Method, rbErr)
			}
		}()
		result, err = jp.Proceed(ctx)
		if err != nil {
			return nil, err
		}
		if cErr := tm.Commit(tx); cErr != nil {
			return nil, fmt.Errorf("aspect: commit transaction for %q: %w", jp.Method, cErr)
		}
		committed = true
		return result, nil
	})
}

// Only returns an interceptor that applies inner only when the joinpoint's method
// is one of methods; for any other method it proceeds straight through. It is the
// pointcut equivalent: a way to scope a concern to a subset of the operations a
// chain guards without building a separate chain per method.
func Only(inner Interceptor, methods ...string) Interceptor {
	return InterceptorFunc(func(jp *Joinpoint) (any, error) {
		if slices.Contains(methods, jp.Method) {
			return inner.Intercept(jp)
		}
		return jp.Proceed(jp.Context)
	})
}
