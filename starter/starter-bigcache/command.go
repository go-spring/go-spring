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

// command.go is the "command seam" concept of this starter: the per-operation
// methods of Cache, each routed through the shared observe span and the
// resilience executor (guard). bigcache exposes no hook or plugin point, so the
// command surface is hand-written here — the analog of starter-memcached's
// command.go.
package StarterBigCache

import (
	"context"
	"errors"
	"time"

	"github.com/allegro/bigcache/v3"
)

// guard runs fn under the resilience executor. bigcache.ErrEntryNotFound is a
// cache miss — a normal, expected outcome — so it is treated as success for the
// breaker/retry (mirroring how go-redis treats redis.Nil and gorm treats
// ErrRecordNotFound). A rejection (rate-limited / circuit-open / bulkhead-full)
// is returned as the executor's sentinel error; any other error from fn feeds the
// breaker and may be retried. When governance is off the resolved executor is a
// no-op, so the overhead is a single function call.
func (c *Cache) guard(ctx context.Context, fn func(context.Context) error) error {
	var callErr error
	execErr := c.exec.Execute(ctx, c.resource, func(ctx context.Context) error {
		callErr = fn(ctx)
		if callErr != nil && !errors.Is(callErr, bigcache.ErrEntryNotFound) {
			return callErr // a real failure feeds the breaker/retry
		}
		return nil // success or cache miss
	})
	if execErr != nil {
		return execErr // rejected by protection, or propagated failure
	}
	return callErr
}

func (c *Cache) Get(key string) ([]byte, error) {
	ctx, span := c.obs.start(context.Background(), "get", key)
	start := time.Now()
	var v []byte
	err := c.guard(ctx, func(ctx context.Context) error {
		var e error
		v, e = c.BigCache.Get(key)
		return e
	})
	c.obs.record(ctx, "get", key, start, span, err)
	return v, err
}

func (c *Cache) Set(key string, entry []byte) error {
	ctx, span := c.obs.start(context.Background(), "set", key)
	start := time.Now()
	err := c.guard(ctx, func(ctx context.Context) error {
		return c.BigCache.Set(key, entry)
	})
	c.obs.record(ctx, "set", key, start, span, err)
	return err
}

func (c *Cache) Delete(key string) error {
	ctx, span := c.obs.start(context.Background(), "delete", key)
	start := time.Now()
	err := c.guard(ctx, func(ctx context.Context) error {
		return c.BigCache.Delete(key)
	})
	c.obs.record(ctx, "delete", key, start, span, err)
	return err
}
