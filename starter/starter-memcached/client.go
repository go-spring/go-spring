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

// client.go is the "resource entity" concept of this starter: the Client
// wrapper memcached clients are injected as, its lifecycle (Init/Destroy), and
// the per-operation command surface. It mirrors starter-redigo's pool.go (the
// command surface mirrors its conn.go). Every command method is routed through
// the shared run/runErr seam, which wraps the operation in an observe span and
// the resilience executor — gomemcache exposes no hook or plugin point, so the
// command surface is hand-written.
//
// Every command method takes a ctx. gomemcache itself cannot honor it on the
// wire (the socket wait is bounded by Config.Timeout), but ctx still governs
// cancellation of the resilience layer (rate-limit wait, retry backoff,
// breaker checks) and is inherited by the observe span.
package StarterMemcached

import (
	"context"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client wraps *memcache.Client so every operation flows through the
// module-local observer (trace span + duration metric + access log). It
// embeds the real client, so methods not overridden below (only Close, a
// lifecycle method) are promoted unchanged. Every command method takes a ctx:
// the span inherits it, but gomemcache's wire call cannot honor it (the socket
// wait is bounded by timeout) — a limitation of gomemcache, documented here.
//
// The type is exported because gomemcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Client rather
// than *memcache.Client; the embedded field is available for any third-party
// API that needs the raw client.
type Client struct {
	*memcache.Client

	// obs emits the per-operation span/metric/access-log triple; built by Init.
	obs *observer

	// mgr and inj are the governance beans gs injects into the constructor (both
	// nil in a standalone call). mgr is normalized in Init: an unarmed manager is
	// exactly the "governance off" pass-through, while a nil pointer would panic
	// on the method call. inj is nil-safe at its use site.
	mgr *resilience.Manager
	inj *fault.Injector

	// serviceName is the discovery service-name the entry addresses (empty when
	// directly addressed), and name is the instance name (the
	// spring.memcached.instances.<name> map key). The governance service label
	// prefers the service name — governance targets the downstream service, the
	// same identity every caller of that service shares — and falls back to the
	// instance name. Set by newClient; Init reads them.
	serviceName string
	name        string

	// exec is the resilience executor protecting every operation, resolved from
	// the injected manager; no-op when governance is off.
	exec resilience.ClientExecutor
	// service is the resilience service key ("memcached:<service-name or
	// instance-name>") exec scopes limiter/breaker state by.
	service string
}

// Init is the gs InitMethod. It builds the module-local observer (see
// observe.go) and arms the executor from the injected governance manager. The
// manager's ClientExecutorFor resolves its backing executor lazily, on each Execute, so
// the arming order relative to starter-governance's wiring is irrelevant; the
// fault injector wraps it with inj, which is nil-safe (with no injector the fault
// layer is a transparent pass-through). When governance is off — an unarmed
// manager — the resolved executor is a transparent no-op.
func (c *Client) Init() error {
	c.obs = newObserver()
	c.service = resilience.ServiceLabel("memcached", c.serviceName, c.name)
	c.exec = fault.WrapClientExecutor(c.mgr.ClientExecutorFor("memcached", c.service), c.service, c.inj)
	return nil
}

// Destroy releases the resilience executor (if armed). The memcache client
// itself keeps a lazy connection pool with no Close method and discovery runs
// inside the backend (the loader has no resources), so only the executor's
// services are released here. It is the gs destroy method.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return nil
}

// run routes a payload-bearing operation of client c through the shared seam:
// op names the observe span (key is the log/span arg), fn runs under the
// resilience executor via [resilience.Run], and the span ends with the
// operation's error. A cache miss (resilience.Tolerate) neither trips the
// breaker nor retries; protection rejections (rate-limited / circuit-open /
// bulkhead-full) surface to the caller. When governance is off the resolved
// executor is a no-op, so fn runs with a single function-call overhead.
func run[T any](ctx context.Context, c *Client, op, key string, fn func() (T, error)) (T, error) {
	sp := c.obs.Start(ctx, op, key)
	t, err := resilience.Run(ctx, c.exec,
		func(context.Context) (T, error) { return fn() },
		resilience.Tolerate(memcache.ErrCacheMiss))
	sp.End(err)
	return t, err
}

// runErr is the error-only variant of [run], for operations that return no
// payload (Set/Delete/Ping/...). It shares the same span + resilience +
// ErrCacheMiss semantics; the two-variant split keeps the call sites typed
// rather than routing through any + runtime assertion.
func runErr(ctx context.Context, c *Client, op, key string, fn func() error) error {
	_, err := run(ctx, c, op, key, func() (struct{}, error) { return struct{}{}, fn() })
	return err
}

// Get returns the item for key. Returns memcache.ErrCacheMiss when the key is
// not present.
func (c *Client) Get(ctx context.Context, key string) (*memcache.Item, error) {
	return run(ctx, c, "get", key, func() (*memcache.Item, error) { return c.Client.Get(key) })
}

// GetAndTouch returns the item for key and updates its expiration. seconds is
// the new expiration time, in seconds.
func (c *Client) GetAndTouch(ctx context.Context, key string, seconds int32) (*memcache.Item, error) {
	return run(ctx, c, "get_and_touch", key, func() (*memcache.Item, error) {
		return c.Client.GetAndTouch(key, seconds)
	})
}

// GetMulti fetches the items for keys in one round trip. Missing keys are
// simply absent from the returned map; keys absent from the map are cache
// misses, not errors.
func (c *Client) GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error) {
	return run(ctx, c, "get_multi", fmt.Sprintf("%d keys", len(keys)), func() (map[string]*memcache.Item, error) {
		return c.Client.GetMulti(keys)
	})
}

// Touch updates the expiration time of the item for key without fetching it.
// seconds is the new expiration time, in seconds.
func (c *Client) Touch(ctx context.Context, key string, seconds int32) error {
	return runErr(ctx, c, "touch", key, func() error { return c.Client.Touch(key, seconds) })
}

// Set stores item, overwriting any existing value for its key.
func (c *Client) Set(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "set", item.Key, func() error { return c.Client.Set(item) })
}

// Add stores item only when its key holds no value yet; it returns
// memcache.ErrNotStored otherwise.
func (c *Client) Add(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "add", item.Key, func() error { return c.Client.Add(item) })
}

// Replace overwrites the value of item's key only when it already holds a
// value; it returns memcache.ErrNotStored otherwise.
func (c *Client) Replace(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "replace", item.Key, func() error { return c.Client.Replace(item) })
}

// Append appends item's value to the value already stored under its key.
func (c *Client) Append(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "append", item.Key, func() error { return c.Client.Append(item) })
}

// Prepend prepends item's value to the value already stored under its key.
func (c *Client) Prepend(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "prepend", item.Key, func() error { return c.Client.Prepend(item) })
}

// CompareAndSwap stores item only when its cas token (from a prior Get)
// still matches the stored value; it returns memcache.ErrCASConflict on a
// mismatch.
func (c *Client) CompareAndSwap(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "cas", item.Key, func() error { return c.Client.CompareAndSwap(item) })
}

// Delete removes the value stored under key. Returns memcache.ErrCacheMiss
// when the key holds no value.
func (c *Client) Delete(ctx context.Context, key string) error {
	return runErr(ctx, c, "delete", key, func() error { return c.Client.Delete(key) })
}

// DeleteAll issues the memcached "flush_all" command to a single server in
// the pool (the one the empty key hashes to). To invalidate every server, use
// FlushAll.
func (c *Client) DeleteAll(ctx context.Context) error {
	return runErr(ctx, c, "delete_all", "", func() error { return c.Client.DeleteAll() })
}

// Increment adds delta to the uint64 counter stored under key and returns the
// new value. Returns memcache.ErrCacheMiss when the key holds no value.
func (c *Client) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	return run(ctx, c, "increment", key, func() (uint64, error) { return c.Client.Increment(key, delta) })
}

// Decrement subtracts delta from the uint64 counter stored under key and
// returns the new value. Returns memcache.ErrCacheMiss when the key holds no
// value.
func (c *Client) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	return run(ctx, c, "decrement", key, func() (uint64, error) { return c.Client.Decrement(key, delta) })
}

// Ping tests the connectivity of every server in the pool with a no-op
// request.
func (c *Client) Ping(ctx context.Context) error {
	return runErr(ctx, c, "ping", "", func() error { return c.Client.Ping() })
}

// FlushAll issues the memcached "flush_all" command to every server in the
// pool, invalidating all items on each.
func (c *Client) FlushAll(ctx context.Context) error {
	return runErr(ctx, c, "flush_all", "", func() error { return c.Client.FlushAll() })
}
