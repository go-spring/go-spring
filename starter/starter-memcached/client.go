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
// wrapper memcached clients are injected as, its lifecycle (Init/Close), and
// the InnerClient chain the command surface runs through. gomemcache exposes
// no hook or plugin point and delivers a concrete type, so the chain is the
// only seam: the executor layer declares each operation's identity and runs it
// under governance, the adapter layer discards the context, and a custom layer
// may sit between them.
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
	"go-spring.org/cloud"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
)

// Client wraps a memcache client so every operation carries its semantic
// identity and flows through the governance executor, which is also where the
// operation's span, metrics and access log are emitted. The type holds exactly
// two exported things, both deliberate: the embedded [InnerClient] the command
// surface runs through — reorganized by wrapping the current head in a layer
// of your own, a plain assignment in reviewable sight — and [Client.Client],
// the raw instance. [NewClient] is the only way to build one, so a client can
// never exist without its identity.
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
	// The embedded InnerClient is the chain the command surface runs through,
	// exposed so a driver's post-processing (or the app, right after wiring)
	// can reorganize it: wrap the current head in its own [InnerClient] layer,
	// and the chain below — identity, governance, adapter — keeps doing its
	// job under the layer: the keys the layer rewrites are what the identity
	// layer declares, and the executor still protects every call. Rebuilding
	// is a build-time move — write it before the client takes traffic, not
	// racing it.
	InnerClient

	// Client is the raw gomemcache client this wrapper bottoms out in — the
	// original object, not a wrapper. A handle to READ (and to hand to the
	// chain constructors when a driver builds its own assembly), never to
	// reassign or to run commands through — that would bypass the governance
	// and observation layers.
	Client *memcache.Client
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
	raw := NewRawClient(client)
	guard := NewGuardClient(raw, instanceName, serviceName, params)
	return &Client{Client: client, InnerClient: NewObsClient(guard)}
}

// Close tears the client down through its chain — the head's Release(true),
// layer by layer, the connection pool last — and is also the gs destroy
// method, registered on the bean. The pool close comes last on purpose: once
// the executor is gone the client is no longer protected, and there is no
// reason to keep sockets alive past that point.
func (c *Client) Close() error { return c.InnerClient.Release(true) }

// InnerClient is the command seam the client's internals are hollowed into:
// the business traffic gomemcache carries, each call carrying its context.
// gomemcache offers no hook or plugin point, so this interface is the ONLY way
// to modify what happens under the promoted command methods.
//
// The default chain is the identity layer over the governance layer over a
// raw adapter, and the embedded InnerClient is where a custom layer goes:
// implement this interface (embed the head you found to inherit the methods
// you do not care about), then assign your layer over it. The chain under the
// layer keeps doing its job — the keys the layer rewrites are what the
// identity layer declares, and the executor still protects every call —
// which is usually the point of adding it.
//
// Release tears the chain down, layer by layer, and releaseRaw picks how far
// down: every layer takes away its OWN resources and passes the flag to the
// layer under it unchanged, and only the instance layer at the bottom acts on
// the flag — closing the raw client when it is set. Release(false) is the
// shallow release — every layer's own cleanup, the instance left running;
// Release(true) is the full teardown, and [Client.Close] is nothing but the
// head's Release(true).
type InnerClient interface {
	// Get returns the item for key, or memcache.ErrCacheMiss when absent.
	Get(ctx context.Context, key string) (*memcache.Item, error)
	// GetAndTouch returns the item for key and updates its expiration.
	GetAndTouch(ctx context.Context, key string, seconds int32) (*memcache.Item, error)
	// GetMulti fetches the items for keys in one round trip; missing keys are
	// absent from the map.
	GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error)
	// Touch updates the expiration of key without fetching it.
	Touch(ctx context.Context, key string, seconds int32) error
	// Set stores item, overwriting any existing value for its key.
	Set(ctx context.Context, item *memcache.Item) error
	// Add stores item only when its key holds no value yet.
	Add(ctx context.Context, item *memcache.Item) error
	// Replace overwrites item's key only when it already holds a value.
	Replace(ctx context.Context, item *memcache.Item) error
	// Append appends item's value to the value stored under its key.
	Append(ctx context.Context, item *memcache.Item) error
	// Prepend prepends item's value to the value stored under its key.
	Prepend(ctx context.Context, item *memcache.Item) error
	// CompareAndSwap stores item only when its cas token still matches.
	CompareAndSwap(ctx context.Context, item *memcache.Item) error
	// Delete removes the value stored under key.
	Delete(ctx context.Context, key string) error
	// DeleteAll issues "flush_all" to a single server in the pool.
	DeleteAll(ctx context.Context) error
	// Increment adds delta to the counter stored under key.
	Increment(ctx context.Context, key string, delta uint64) (uint64, error)
	// Decrement subtracts delta from the counter stored under key.
	Decrement(ctx context.Context, key string, delta uint64) (uint64, error)
	// Ping tests the connectivity of every server with a no-op request.
	Ping(ctx context.Context) error
	// FlushAll issues "flush_all" to every server in the pool.
	FlushAll(ctx context.Context) error
	// Release releases the layer's own resources, then hands releaseRaw to
	// the layer under it.
	Release(releaseRaw bool) error
}

// RawClient adapts the raw gomemcache client to [InnerClient]: it is the tail
// of the default chain, discarding the context — gomemcache's own API has
// none — and returning its errors verbatim. It is a plain adapter: no
// governance or observability of its own lives here, and no resource but the
// client itself. [NewRawClient] builds it.
type RawClient struct{ raw *memcache.Client }

// NewRawClient wraps a raw gomemcache client as the [InnerClient] tail of a
// chain.
func NewRawClient(raw *memcache.Client) *RawClient { return &RawClient{raw: raw} }

// Release closes the raw client's idle connection pool — and only with
// releaseRaw: the instance layer has no resource of its own, so the shallow
// release is a no-op and the client keeps running. gomemcache documents that
// the client stays usable after a close, so a full teardown's pool close is
// safe even after earlier Close calls.
func (r *RawClient) Release(releaseRaw bool) error {
	if !releaseRaw {
		return nil
	}
	return r.raw.Close()
}

func (r *RawClient) Get(_ context.Context, key string) (*memcache.Item, error) {
	return r.raw.Get(key)
}

func (r *RawClient) GetAndTouch(_ context.Context, key string, seconds int32) (*memcache.Item, error) {
	return r.raw.GetAndTouch(key, seconds)
}

func (r *RawClient) GetMulti(_ context.Context, keys []string) (map[string]*memcache.Item, error) {
	return r.raw.GetMulti(keys)
}

func (r *RawClient) Touch(_ context.Context, key string, seconds int32) error {
	return r.raw.Touch(key, seconds)
}

func (r *RawClient) Set(_ context.Context, item *memcache.Item) error { return r.raw.Set(item) }

func (r *RawClient) Add(_ context.Context, item *memcache.Item) error { return r.raw.Add(item) }

func (r *RawClient) Replace(_ context.Context, item *memcache.Item) error {
	return r.raw.Replace(item)
}

func (r *RawClient) Append(_ context.Context, item *memcache.Item) error {
	return r.raw.Append(item)
}

func (r *RawClient) Prepend(_ context.Context, item *memcache.Item) error {
	return r.raw.Prepend(item)
}

func (r *RawClient) CompareAndSwap(_ context.Context, item *memcache.Item) error {
	return r.raw.CompareAndSwap(item)
}

func (r *RawClient) Delete(_ context.Context, key string) error { return r.raw.Delete(key) }

func (r *RawClient) DeleteAll(_ context.Context) error { return r.raw.DeleteAll() }

func (r *RawClient) Increment(_ context.Context, key string, delta uint64) (uint64, error) {
	return r.raw.Increment(key, delta)
}

func (r *RawClient) Decrement(_ context.Context, key string, delta uint64) (uint64, error) {
	return r.raw.Decrement(key, delta)
}

func (r *RawClient) Ping(_ context.Context) error { return r.raw.Ping() }

func (r *RawClient) FlushAll(_ context.Context) error { return r.raw.FlushAll() }

// ObsClient is the identity layer at the head of the default chain: it names
// each call — op and key, declared as the call's semantic identity via
// [observability.WithOperation] — and hands the context down. It emits nothing
// itself and holds no resource: emission happens where the whole call is seen,
// retries included, which for this starter is the governance layer under it.
// This layer is the analog of starter-bigcache's observation layer — the
// vocabulary lives in observe.go — and is what makes a custom layer's behavior
// (the keys it rewrote, the time it took) visible in the signals.
// [NewObsClient] builds it.
type ObsClient struct {
	next InnerClient
}

// NewObsClient builds the identity layer over next.
func NewObsClient(next InnerClient) *ObsClient { return &ObsClient{next: next} }

// Release hands releaseRaw to the layer under it — this layer holds no
// resource, so there is nothing to take away at either depth.
func (o *ObsClient) Release(releaseRaw bool) error { return o.next.Release(releaseRaw) }

// withOp declares one call's semantic identity: op names the command and key
// is its argument, neither ever a metric label (see observe.go for the
// vocabulary and why the key rides in Detail only).
func withOp(ctx context.Context, op, key string) context.Context {
	return observability.WithOperation(ctx, operation(op, key))
}

func (o *ObsClient) Get(ctx context.Context, key string) (*memcache.Item, error) {
	return o.next.Get(withOp(ctx, "get", key), key)
}

func (o *ObsClient) GetAndTouch(ctx context.Context, key string, seconds int32) (*memcache.Item, error) {
	return o.next.GetAndTouch(withOp(ctx, "get_and_touch", key), key, seconds)
}

func (o *ObsClient) GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error) {
	return o.next.GetMulti(withOp(ctx, "get_multi", fmt.Sprintf("%d keys", len(keys))), keys)
}

func (o *ObsClient) Touch(ctx context.Context, key string, seconds int32) error {
	return o.next.Touch(withOp(ctx, "touch", key), key, seconds)
}

func (o *ObsClient) Set(ctx context.Context, item *memcache.Item) error {
	return o.next.Set(withOp(ctx, "set", item.Key), item)
}

func (o *ObsClient) Add(ctx context.Context, item *memcache.Item) error {
	return o.next.Add(withOp(ctx, "add", item.Key), item)
}

func (o *ObsClient) Replace(ctx context.Context, item *memcache.Item) error {
	return o.next.Replace(withOp(ctx, "replace", item.Key), item)
}

func (o *ObsClient) Append(ctx context.Context, item *memcache.Item) error {
	return o.next.Append(withOp(ctx, "append", item.Key), item)
}

func (o *ObsClient) Prepend(ctx context.Context, item *memcache.Item) error {
	return o.next.Prepend(withOp(ctx, "prepend", item.Key), item)
}

func (o *ObsClient) CompareAndSwap(ctx context.Context, item *memcache.Item) error {
	return o.next.CompareAndSwap(withOp(ctx, "cas", item.Key), item)
}

func (o *ObsClient) Delete(ctx context.Context, key string) error {
	return o.next.Delete(withOp(ctx, "delete", key), key)
}

func (o *ObsClient) DeleteAll(ctx context.Context) error {
	return o.next.DeleteAll(withOp(ctx, "delete_all", ""))
}

func (o *ObsClient) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	return o.next.Increment(withOp(ctx, "increment", key), key, delta)
}

func (o *ObsClient) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	return o.next.Decrement(withOp(ctx, "decrement", key), key, delta)
}

func (o *ObsClient) Ping(ctx context.Context) error {
	return o.next.Ping(withOp(ctx, "ping", ""))
}

func (o *ObsClient) FlushAll(ctx context.Context) error {
	return o.next.FlushAll(withOp(ctx, "flush_all", ""))
}

// GuardClient is the governance layer of the default chain: it runs every
// operation under the resilience executor, which applies rate limiting,
// breaking and bulkheading — and emits the span, duration metrics and access
// log from the one point that sees the whole call, retries included. A cache
// miss (resilience.Tolerate) neither trips the breaker nor retries; protection
// rejections (rate-limited / circuit-open / bulkhead-full) surface to the
// caller. bigcache has no such layer — it is in-process, with nothing to
// protect — which is exactly the RPC/non-RPC split this layer embodies. When
// governance is off the resolved executor is a no-op, so the call runs with a
// single function-call overhead. [NewGuardClient] builds it.
type GuardClient struct {
	exec chain.Executor
	next InnerClient
}

// NewGuardClient builds the governance layer over next, constructing its own
// executor along the way: params carries the container's facilities (see
// [cloud.ClientParams]) and instanceName/serviceName fix the governance label
// ("memcached:<service-name or instance-name>") its limiter and breaker state
// scope by. The executor is the layer's own business end to end — built here,
// used here, closed at the layer's Release — and never leaves it.
func NewGuardClient(next InnerClient, instanceName, serviceName string, params cloud.ClientParams) *GuardClient {
	label := resilience.ServiceLabel("memcached", serviceName, instanceName)
	return &GuardClient{exec: params.ExecutorFor("memcached", label), next: next}
}

// guard runs one payload under the executor with the cache-miss tolerance. It
// is a free function because its generic parameter cannot live on a method.
func guard[T any](ctx context.Context, exec chain.Executor, fn func(context.Context) (T, error)) (T, error) {
	return resilience.Run(ctx, exec, fn, resilience.Tolerate(memcache.ErrCacheMiss))
}

// guardErr is the error-only variant of [guard].
func guardErr(ctx context.Context, exec chain.Executor, fn func(context.Context) error) error {
	_, err := guard(ctx, exec, func(ctx context.Context) (struct{}, error) { return struct{}{}, fn(ctx) })
	return err
}

// Release hands releaseRaw to the layer under it, and on the FULL teardown
// takes down the executor — the layer's own resource, closed only then
// because a rebuilt chain's guard reuses this same executor; the shallow
// release leaves both the executor and the instance running.
func (g *GuardClient) Release(releaseRaw bool) error {
	err := g.next.Release(releaseRaw)
	if releaseRaw && g.exec != nil {
		_ = g.exec.Close()
	}
	return err
}

func (g *GuardClient) Get(ctx context.Context, key string) (*memcache.Item, error) {
	return guard(ctx, g.exec, func(context.Context) (*memcache.Item, error) { return g.next.Get(ctx, key) })
}

func (g *GuardClient) GetAndTouch(ctx context.Context, key string, seconds int32) (*memcache.Item, error) {
	return guard(ctx, g.exec, func(context.Context) (*memcache.Item, error) { return g.next.GetAndTouch(ctx, key, seconds) })
}

func (g *GuardClient) GetMulti(ctx context.Context, keys []string) (map[string]*memcache.Item, error) {
	return guard(ctx, g.exec, func(context.Context) (map[string]*memcache.Item, error) { return g.next.GetMulti(ctx, keys) })
}

func (g *GuardClient) Touch(ctx context.Context, key string, seconds int32) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Touch(ctx, key, seconds) })
}

func (g *GuardClient) Set(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Set(ctx, item) })
}

func (g *GuardClient) Add(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Add(ctx, item) })
}

func (g *GuardClient) Replace(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Replace(ctx, item) })
}

func (g *GuardClient) Append(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Append(ctx, item) })
}

func (g *GuardClient) Prepend(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Prepend(ctx, item) })
}

func (g *GuardClient) CompareAndSwap(ctx context.Context, item *memcache.Item) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.CompareAndSwap(ctx, item) })
}

func (g *GuardClient) Delete(ctx context.Context, key string) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Delete(ctx, key) })
}

func (g *GuardClient) DeleteAll(ctx context.Context) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.DeleteAll(ctx) })
}

func (g *GuardClient) Increment(ctx context.Context, key string, delta uint64) (uint64, error) {
	return guard(ctx, g.exec, func(context.Context) (uint64, error) { return g.next.Increment(ctx, key, delta) })
}

func (g *GuardClient) Decrement(ctx context.Context, key string, delta uint64) (uint64, error) {
	return guard(ctx, g.exec, func(context.Context) (uint64, error) { return g.next.Decrement(ctx, key, delta) })
}

func (g *GuardClient) Ping(ctx context.Context) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.Ping(ctx) })
}

func (g *GuardClient) FlushAll(ctx context.Context) error {
	return guardErr(ctx, g.exec, func(context.Context) error { return g.next.FlushAll(ctx) })
}
