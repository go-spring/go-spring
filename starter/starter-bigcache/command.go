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
// methods of Cache, each routed through the shared run/runErr seam — which
// declares the operation's identity and threads it through the resilience
// executor (guard). bigcache exposes no hook or plugin point, so the command
// surface is hand-written here — the analog of starter-memcached's command.go.
package StarterBigCache

import (
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// run routes one bigcache operation through the shared seam: op names the
// command and key is its argument, both declared as the call's semantic
// identity; fn then runs under the resilience executor via [resilience.Run].
//
// bigcache.ErrEntryNotFound is a cache miss — a normal, expected outcome — so it
// is declared with [resilience.Tolerate]: it neither trips the breaker nor
// retries, mirroring how go-redis treats redis.Nil and gorm treats
// ErrRecordNotFound, while still being returned to the caller verbatim. A
// rejection (rate-limited / circuit-open / bulkhead-full) surfaces to the caller.
// When governance is off the resolved executor is a no-op, so fn runs with a
// single function-call overhead.
//
// The span, duration metrics and access log are not emitted here: declaring the
// identity is this layer's whole job, and the resilience layer emits from the one
// point on the chain that sees the whole call, retries included.
func run[T any](c *Cache, op, key string, fn func() (T, error)) (T, error) {
	ctx := observability.WithOperation(context.Background(), operation(op, key))
	return resilience.Run(ctx, c.exec,
		func(context.Context) (T, error) { return fn() },
		resilience.Tolerate(bigcache.ErrEntryNotFound))
}

// runErr is the error-only variant of [run], for operations that return no
// payload (Set/Delete). It shares the same declaration + resilience + cache-miss
// semantics; the two-variant split keeps the call sites typed rather than
// routing through any + runtime assertion.
func runErr(c *Cache, op, key string, fn func() error) error {
	_, err := run(c, op, key, func() (struct{}, error) { return struct{}{}, fn() })
	return err
}

func (c *Cache) Get(key string) ([]byte, error) {
	return run(c, "get", key, func() ([]byte, error) { return c.client.Get(key) })
}

func (c *Cache) Set(key string, entry []byte) error {
	return runErr(c, "set", key, func() error { return c.client.Set(key, entry) })
}

func (c *Cache) Delete(key string) error {
	return runErr(c, "delete", key, func() error { return c.client.Delete(key) })
}
