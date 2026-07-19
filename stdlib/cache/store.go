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

package cache

import (
	"context"
	"time"

	"go-spring.org/stdlib/aspect"
)

// storeAdapter bridges a [Cache] to the minimal [aspect.Store] seam that the
// aspect Cache interceptor reads and writes.
type storeAdapter struct {
	cache Cache
	ctx   context.Context
}

// AsStore adapts a [Cache] to [aspect.Store], so the aspect Cache marker
// (the @Cacheable equivalent) can be backed by any cache backend registered
// here — in-process, Redis, memcached, or a [MultiLevel] combination — without
// changing the interceptor.
//
// aspect.Store is deliberately context-free (a method interceptor caches by
// logical key, not by request), so lookups run under ctx; pass
// context.Background when there is no ambient one. A backend error is treated
// as a miss (aspect then proceeds to the target), matching aspect's
// fail-open caching contract.
func AsStore(ctx context.Context, c Cache) aspect.Store {
	if ctx == nil {
		ctx = context.Background()
	}
	return &storeAdapter{cache: c, ctx: ctx}
}

// Get implements [aspect.Store]. A backend error is reported as a miss so the
// interceptor falls through to the target rather than failing the call.
func (s *storeAdapter) Get(key string) (any, bool) {
	v, ok, err := s.cache.Get(s.ctx, key)
	if err != nil {
		return nil, false
	}
	return v, ok
}

// Set implements [aspect.Store]. A backend error is swallowed: a failed cache
// write must not fail the operation whose result it was caching.
func (s *storeAdapter) Set(key string, val any, ttl time.Duration) {
	_ = s.cache.Set(s.ctx, key, val, ttl)
}
