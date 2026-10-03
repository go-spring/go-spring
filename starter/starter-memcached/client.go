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
	"go-spring.org/cloud/chain"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// Client wraps a memcache client so every operation carries its semantic
// identity and flows through the governance executor, which is also where the
// operation's span, metrics and access log are emitted. The raw client is an
// unexported field, not an embedded
// one: [NewClient] is the only way to build a Client, so a client can never
// exist without its identity, and the command surface below is the whole API.
// There is deliberately no exported accessor for the raw client: that would let
// a caller bypass the observe and governance layers without it showing up in
// review.
//
// Every command method takes a ctx: the span inherits it, but gomemcache's wire
// call cannot honor it (the socket wait is bounded by timeout) — a limitation
// of gomemcache, documented here.
//
// The type is exported because gomemcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Client rather than
// *memcache.Client. [Driver.CreateClient] returns this type too, so a custom
// driver works with the same type the ecosystem sees.
type Client struct {
	// client is the raw gomemcache client. Unexported so [NewClient] is the
	// only constructor and nothing outside this package can reach it — see the
	// type doc.
	client *memcache.Client

	// serviceName is the discovery service-name the entry addresses (empty when
	// directly addressed), and instanceName is the instance name (the
	// spring.memcached.instances.<instanceName> map key). The governance service
	// label prefers the service name — governance targets the downstream service,
	// the same identity every caller of that service shares — and falls back to
	// the instance name. Both are fixed by [NewClient].
	serviceName  string
	instanceName string

	// exec is the resilience executor protecting every operation, applied by
	// [NewClient] while it builds; no-op when governance is off.
	exec chain.Executor
	// serviceLabel is the resilience service key ("memcached:<service-name or
	// instance-name>") exec scopes limiter/breaker state by.
	serviceLabel string
}

// NewClient builds a complete Client — identity, governance and all — over a
// connected raw client. client must be ready for use (dialed, with timeouts
// applied) — it is normally the Driver's product. instanceName is the config
// entry's key and serviceName its discovery name; both may be empty when the
// entry addresses a fixed server list.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client passes the zero [cloud.ClientParams]; its executor then
// degrades to resilience.Unmanaged — observed, with a one-time warning that no
// protection applies — rather than silently running bare.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(client *memcache.Client, instanceName, serviceName string, params cloud.ClientParams) *Client {
	c := &Client{
		client:       client,
		instanceName: instanceName,
		serviceName:  serviceName,
	}
	c.serviceLabel = resilience.ServiceLabel("memcached", c.serviceName, c.instanceName)
	c.exec = params.ExecutorFor("memcached", c.serviceLabel)
	return c
}

// Close closes the client's idle connections. It exists because the raw client
// has it: the field is unexported, so nothing is promoted and the method would
// otherwise be reachable only through [Client.Destroy]. gomemcache documents
// that the client stays usable after Close, so calling it here and again from
// Destroy is safe.
func (c *Client) Close() error { return c.client.Close() }

// Destroy releases the resilience executor (if governance was applied) and
// closes the client's idle connection pool. Discovery runs inside the backend
// (the loader has no resources), so there is nothing else to release. It is the
// gs destroy method.
//
// The pool close comes first: once the executor is gone the client is no longer
// protected, and there is no reason to keep sockets alive past that point.
// Returns the pool's error; the executor's Close is best-effort.
func (c *Client) Destroy() error {
	err := c.client.Close()
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return err
}

// run routes a payload-bearing operation of client c through the shared seam:
// op names the command and key is its argument, both declared as the call's
// semantic identity; fn then runs under the resilience executor via
// [resilience.Run]. A cache miss (resilience.Tolerate) neither trips the breaker
// nor retries; protection rejections (rate-limited / circuit-open /
// bulkhead-full) surface to the caller. When governance is off the resolved
// executor is a no-op, so fn runs with a single function-call overhead.
//
// The span, duration metrics and access log are not emitted here: declaring the
// identity is this layer's whole job now, and the resilience layer emits from
// the one point on the chain that sees the whole call, retries included.
func run[T any](ctx context.Context, c *Client, op, key string, fn func() (T, error)) (T, error) {
	ctx = observability.WithOperation(ctx, operation(op, key))
	return resilience.Run(ctx, c.exec,
		func(context.Context) (T, error) { return fn() },
		resilience.Tolerate(memcache.ErrCacheMiss))
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
	return run(ctx, c, "get", key, func() (*memcache.Item, error) { return c.client.Get(key) })
}

// GetAndTouch returns the item for key and updates its expiration. seconds is
// the new expiration time, in seconds.
func (c *Client) GetAndTouch(ctx context.Context, key string, seconds int32) (*memcache.Item, error) {
	return run(ctx, c, "get_and_touch", key, func() (*memcache.Item, error) {
		return c.client.GetAndTouch(key, seconds)
	})
}

// GetMulti fetches the items for keys in one round trip. Missing keys are
// simply absent from the returned map; keys absent from the map are cache
// misses, not errors.
func (c *Client) GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error) {
	return run(ctx, c, "get_multi", fmt.Sprintf("%d keys", len(keys)), func() (map[string]*memcache.Item, error) {
		return c.client.GetMulti(keys)
	})
}

// Touch updates the expiration time of the item for key without fetching it.
// seconds is the new expiration time, in seconds.
func (c *Client) Touch(ctx context.Context, key string, seconds int32) error {
	return runErr(ctx, c, "touch", key, func() error { return c.client.Touch(key, seconds) })
}

// Set stores item, overwriting any existing value for its key.
func (c *Client) Set(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "set", item.Key, func() error { return c.client.Set(item) })
}

// Add stores item only when its key holds no value yet; it returns
// memcache.ErrNotStored otherwise.
func (c *Client) Add(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "add", item.Key, func() error { return c.client.Add(item) })
}

// Replace overwrites the value of item's key only when it already holds a
// value; it returns memcache.ErrNotStored otherwise.
func (c *Client) Replace(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "replace", item.Key, func() error { return c.client.Replace(item) })
}

// Append appends item's value to the value already stored under its key.
func (c *Client) Append(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "append", item.Key, func() error { return c.client.Append(item) })
}

// Prepend prepends item's value to the value already stored under its key.
func (c *Client) Prepend(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "prepend", item.Key, func() error { return c.client.Prepend(item) })
}

// CompareAndSwap stores item only when its cas token (from a prior Get)
// still matches the stored value; it returns memcache.ErrCASConflict on a
// mismatch.
func (c *Client) CompareAndSwap(ctx context.Context, item *memcache.Item) error {
	return runErr(ctx, c, "cas", item.Key, func() error { return c.client.CompareAndSwap(item) })
}

// Delete removes the value stored under key. Returns memcache.ErrCacheMiss
// when the key holds no value.
func (c *Client) Delete(ctx context.Context, key string) error {
	return runErr(ctx, c, "delete", key, func() error { return c.client.Delete(key) })
}

// DeleteAll issues the memcached "flush_all" command to a single server in
// the pool (the one the empty key hashes to). To invalidate every server, use
// FlushAll.
func (c *Client) DeleteAll(ctx context.Context) error {
	return runErr(ctx, c, "delete_all", "", func() error { return c.client.DeleteAll() })
}

// Increment adds delta to the uint64 counter stored under key and returns the
// new value. Returns memcache.ErrCacheMiss when the key holds no value.
func (c *Client) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	return run(ctx, c, "increment", key, func() (uint64, error) { return c.client.Increment(key, delta) })
}

// Decrement subtracts delta from the uint64 counter stored under key and
// returns the new value. Returns memcache.ErrCacheMiss when the key holds no
// value.
func (c *Client) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	return run(ctx, c, "decrement", key, func() (uint64, error) { return c.client.Decrement(key, delta) })
}

// Ping tests the connectivity of every server in the pool with a no-op
// request.
func (c *Client) Ping(ctx context.Context) error {
	return runErr(ctx, c, "ping", "", func() error { return c.client.Ping() })
}

// FlushAll issues the memcached "flush_all" command to every server in the
// pool, invalidating all items on each.
func (c *Client) FlushAll(ctx context.Context) error {
	return runErr(ctx, c, "flush_all", "", func() error { return c.client.FlushAll() })
}
