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

package resilience

import (
	"context"
	"sync"
	"sync/atomic"
)

// Provider builds the resilience [Executor] a given resource should run under.
// It is the bridge between the centralized governance authority (cloud/governance,
// wired by starter-govern) and every client adapter: clients call
// [ExecutorFor] with their resource label and get back an executor whose policy
// the authority owns and hot-reloads — WITHOUT the client having to inject or
// even name the governance type. This keeps cross-cutting resilience out of
// client structs: a client imports only this package (which it already does to
// build executors) and calls a neutral function, never cloud/governance.
//
// The provider is registered once, by starter-govern, after it has built the
// governance center. Resolution is deferred to call time (see [ExecutorFor]),
// so the order in which a client is set up relative to that registration does
// not matter: a request always runs after the whole container has wired.
type Provider func(label string) Executor

var provider atomic.Pointer[Provider]

// cache memoizes the executor each label resolves to, so the provider is
// invoked (and any per-label registration done inside it) at most once per
// label — even though [ExecutorFor] may be called per operation. The first
// [resolvedExecutor].Execute for a label pays the build; every later call,
// from any client, is a sync.Map load. The cache is cleared whenever a new
// provider is registered, so a provider swap (each RunTest rebuilding the
// governance center) never serves executors bound to the old one.
var cache sync.Map // label -> Executor

// RegisterExecutorProvider installs p as the process-wide executor provider.
// starter-govern calls this once after building the governance center; clients
// never call it. Calling more than once replaces the provider and drops every
// memoized executor, so the next resolve rebuilds against the new provider
// (the common case is a single registration, so replacement is only a safety
// valve, not a supported multi-provider feature). It is safe to call
// concurrently with [ExecutorFor]: executors already resolved and held by
// clients keep working — [ExecutorFor] returns lazy wrappers that re-resolve
// per Execute, so they pick up the rebuilt executor on their next call.
func RegisterExecutorProvider(p Provider) {
	if p == nil {
		return
	}
	provider.Store(&p)
	cache.Range(func(key, _ any) bool {
		cache.Delete(key)
		return true
	})
}

// ExecutorFor returns the executor system's resource should run under. It is
// the single call site clients use instead of injecting a governance center:
// pass the owning system (e.g. "redis", "gorm") and the resource label, wrap
// the result with fault as desired, and install it in your command/query path.
//
// The returned executor is fully composed: resolution guards it with the
// registered provider's policy (when there is one) and wraps it with the
// observe layer under system, so clients do not call [WrapExecutor] themselves.
// It resolves its backing executor LAZILY on each Execute: it looks up the
// provider-built executor for the label (memoized after first use) and
// delegates. Because resolution happens at call time — when a real request
// runs, well after the container has wired — it does not matter whether the
// client was set up before or after starter-govern registered the provider.
// When no provider is registered (starter-govern not imported, or governance
// disabled), the policy layer is a transparent no-op: fn runs once, untouched,
// with no rate-limit/breaker overhead, and only the observe layer remains.
//
// Hot-reload is handled by the provider, not the caller: the provider registers
// a governance subscription on the executor it builds, so a policy change
// refreshes that backing executor in place; this lazy wrapper just delegates to
// it on the next call.
func ExecutorFor(system, resource string) Executor {
	return &resolvedExecutor{system: system, label: resource}
}

// resolvedExecutor is the stable, lazily-delegating executor [ExecutorFor]
// returns. It holds the system label and the resource label; the real executor
// is resolved on each Execute via [resolve] (memoized globally per label).
// Refresh is a no-op here because hot-reload is driven on the backing executor
// by the provider.
type resolvedExecutor struct {
	system string
	label  string
}

func (r *resolvedExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	return resolve(r.label, r.system).Execute(ctx, r.label, fn)
}

func (r *resolvedExecutor) Refresh(Policy) error { return nil }

func (r *resolvedExecutor) Close() error { return nil }

// resolve returns the memoized backing executor for label, building it via the
// registered provider on first use. With no provider it uses a shared
// [noopExecutor], so an unmanaged process pays only an atomic load + a sync.Map
// miss per call (and a hit once any executor for that label is cached).
//
// The observe layer is applied HERE, on the still-private executor, before the
// cache publishes it. That ordering is what lets [WrapExecutor] attach its
// breaker listener through the construction-time handshake
// ([BreakerEventListenerSetter]) while the per-resource breakers do not exist
// yet — and are built afterwards against the captured listener — instead of a
// client trying to reach an executor that is already behind a wrapper chain.
func resolve(label, system string) Executor {
	if v, ok := cache.Load(label); ok {
		return v.(Executor)
	}
	var built Executor = noopExecutorInst
	if p := provider.Load(); p != nil && *p != nil {
		if e := (*p)(label); e != nil {
			built = e
		}
	}
	actual, _ := cache.LoadOrStore(label, WrapExecutor(built, system))
	return actual.(Executor)
}

// noopExecutor runs fn once with no protection — the executor a process with no
// registered provider (governance off) gets. It is the zero-cost fallback that
// keeps client code uniform whether or not resilience is configured.
type noopExecutor struct{}

var noopExecutorInst Executor = noopExecutor{}

func (noopExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopExecutor) Refresh(Policy) error { return nil }

func (noopExecutor) Close() error { return nil }
