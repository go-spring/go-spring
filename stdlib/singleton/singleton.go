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

// Package singleton provides a container for a single value that is constructed
// on first use and reused afterwards.
//
// A singleton is declared at package level, and its zero value is ready to use:
//
//	var db singleton.Singleton[*sql.DB]
//
// The constructor runs on the first call to Init, and later calls return the
// value and the error it produced:
//
//	func Init(ctx context.Context) (*sql.DB, error) {
//		return db.Init(func() (*sql.DB, error) {
//			return sql.Open("mysql", dsn)
//		})
//	}
package singleton

import "sync"

// Singleton holds the value produced by a constructor on first use, together
// with the error that constructor returned. Its zero value is ready to use.
type Singleton[T any] struct {
	mu   sync.Mutex
	done bool
	v    T
	err  error
}

// Init runs newFn on the first call, caches the value and error it returns, and
// returns that pair. Later calls skip newFn and return the cached pair.
//
// Init is safe for concurrent use. The lock is held while newFn runs, so newFn
// must not call Init on the same Singleton, directly or indirectly: the lock is
// not reentrant and that deadlocks.
func (s *Singleton[T]) Init(newFn func() (T, error)) (T, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.v, s.err = newFn()
		s.done = true
	}
	return s.v, s.err
}
