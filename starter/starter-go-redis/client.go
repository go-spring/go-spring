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

// client.go is the "resource entity" concept of this starter — the Client
// wrapper go-redis clients are injected as, plus its lifecycle (NewClient /
// Destroy) and service label — together with the "command seam" that protects
// each command. It mirrors starter-redigo's pool.go and conn.go together: the
// entity embeds the raw client and carries the resilience executor + the
// endpoint-selection subscription, while the per-command hook layers (the
// declaration layer in observe.go and the executor below) ride the client's
// hook chain.

package StarterGoRedis

import (
	"context"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean go-redis clients are injected as. It EMBEDS the
// concrete redis.UniversalClient (a *redis.Client or *redis.ClusterClient
// depending on mode), so [Client] IS a redis.UniversalClient and the whole
// command surface is promoted with no per-method forwarding. The wrapper is a
// holder, not a delegator: go-redis carries its governance extension point
// (AddHook) on the raw client itself, so there is nothing to forward —
// [NewClient] fixes the client's identity, installs the declaration layer, and
// appends the resilience hook there in one step, and the promoted commands run
// through them.
//
// [Driver.CreateClient] returns this type too, so a company Driver works with
// the same type the rest of the ecosystem sees.
type Client struct {
	// UniversalClient is the raw go-redis client the wrapper embeds. It is the
	// real connection; the declaration layer and the resilience hook are attached
	// to it by [NewClient].
	redis.UniversalClient

	// cfg is the entry's bound configuration; its address fields feed the
	// governance service label (see [serviceLabel]).
	cfg Config

	// exec is the resilience executor protecting every command, set by [NewClient]
	// from the container's params bundle. It is never nil: a bundle with no
	// resilience manager degrades to the observed-only
	// [resilience.Unmanaged] executor.
	exec chain.Executor
	// serviceLabel is the resilience service key ("redis:<...>") exec scopes
	// limiter/breaker state by. Fixed by [NewClient].
	serviceLabel string

	// lbPool is the endpoint-selection pool the Driver built and handed over;
	// nil when the topology has no per-endpoint pick (sentinel, cluster, or a
	// static Addr). [NewClient] binds it to the bundle's loadbalance authority and
	// keeps the detach for [Client.Destroy].
	lbPool *loadbalance.Pool
	// detach releases the endpoint-selection subscription; nil when no pool was
	// bound.
	detach func()
}

// NewClient builds a complete Client — identity, observation and governance — over
// a connected raw client. client must be ready for use (dialing and timeouts
// configured) — it is normally the Driver's product. cfg supplies the address
// fields the service label is derived from; lbPool is the pick pool the Driver
// built for a discovery-routed entry, or nil when it built none.
//
// params carries the container's facilities (see [cloud.ClientParams]) and is
// applied HERE, so a Client cannot exist half-assembled: [Client.exec] and the
// pick-pool binding are set in the constructor, and there is no Init step and
// nothing the container has to remember to call later. A hand-built client
// passes the zero [cloud.ClientParams]; its executor then degrades to
// [resilience.Unmanaged] — observed, with a one-time warning that no protection
// applies — rather than silently running bare.
//
// Only the redisotel instrumentation can fail (an SDK misconfiguration); when it
// does, the raw client is closed and the constructor returns the error, so a
// caller never receives a half-observed client.
//
// Layer order (go-redis hooks are FIFO — first added is outermost) is a semantic
// contract shared by the whole client-starter family:
//
//	redisotel (pool metrics)         — added by instrument, outermost
//	operationHook (declaration)      — attached here, outside the breaker
//	resilienceHook (breaker/retry + emission) — attached here, innermost
//
// The declaration therefore reaches the resilience layer, which emits the one
// span, the duration metrics and the access log from the point that sees the
// whole call — so one log line covers the whole retry loop.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(client redis.UniversalClient, cfg Config, lbPool *loadbalance.Pool, params cloud.ClientParams) (*Client, error) {
	c := &Client{UniversalClient: client, cfg: cfg, lbPool: lbPool}
	// Install the declaration layer after the redisotel pool metrics: the
	// declaration must be on the client before any command runs, so it belongs
	// to construction, not to a later assembly step.
	if err := instrument(client, cfg.Otel); err != nil {
		_ = client.Close()
		return nil, err
	}
	applyDeclaration(client)
	// Governance is applied here, in the same act as identity and observation:
	// the service label is fixed, the executor comes from the bundle, and the
	// pick pool — when the Driver built one — is bound to the bundle's loadbalance
	// authority. Never reaching here is impossible: the bundle is the only way the
	// container contributes protection, and a zero bundle degrades to an
	// observed-only executor rather than a bare client.
	c.serviceLabel = serviceLabel(c.cfg)
	c.exec = params.ExecutorFor("redis", c.serviceLabel)
	// A nil pool (a topology with no per-endpoint pick, e.g. sentinel or cluster
	// mode) binds nothing; so does a bundle with no loadbalance authority (a
	// hand-built client). Binding before the manager has been applied is safe —
	// the subscription is remembered and applied once the center
	// goes live — so this is correct at construction time, before the container
	// has finished wiring.
	if c.lbPool != nil && params.Loadbalance != nil {
		c.detach = params.Loadbalance.Bind(c.lbPool, c.serviceLabel)
	}
	// The resilience hook is innermost: it is added after the declaration, so the
	// identity the declaration puts on the ctx is already there when the
	// resilience layer reads it and emits the call's span, metrics and access log.
	c.UniversalClient.AddHook(&resilienceHook{exec: c.exec, serviceLabel: c.serviceLabel})
	return c, nil
}

// Destroy is the gs destroy method: releases the resilience executor, detaches
// the endpoint-selection subscription (when one was bound), and closes the
// underlying client. Discovery runs inside the backend (the resolver owns no
// resources), so there is nothing else to release.
//
// The client close comes last: once the executor is gone the client is no longer
// protected, and there is no reason to keep connections alive past that point.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	if c.detach != nil {
		c.detach()
	}
	return c.UniversalClient.Close()
}

// serviceLabel derives a stable, human-readable resilience service key for a
// client, so limiter and breaker state is scoped per Redis instance rather than
// per command. It falls back across the mode-specific address fields via the
// shared [resilience.ServiceLabel] helper; unlike a memcached entry (whose label
// has no address to fall back on and so needs the instance name), a redis entry
// always carries an address or a service name, so the fallback chain needs no
// instances-map key.
func serviceLabel(c Config) string {
	first := ""
	if len(c.Addrs) > 0 {
		first = c.Addrs[0]
	}
	return resilience.ServiceLabel("redis", c.ServiceName, c.MasterName, c.Addr, first)
}

// resilienceHook is the command seam: the go-redis [redis.Hook] layer that
// routes every Redis command (and pipeline) through the executor. Two hooks ride
// the client's hook chain (FIFO, first added outermost):
//
//	operationHook   — the declaration layer: puts the command's identity on the
//	                  ctx (see observe.go); emits nothing.
//	resilienceHook  — the breaker/retry/rate-limit executor (innermost), which
//	                  is also the single emitter of the call's span, metrics
//	                  and access log.
//
// Their relative order is established at construction (see [NewClient]) and is a
// semantic contract: the declaration sits outside the breaker, so the identity
// reaches the resilience layer and one log line covers the whole retry loop.
//
// DialHook is left untouched — connection establishment is discovery's concern,
// not the command-level protection we add here.
type resilienceHook struct {
	exec         chain.Executor
	serviceLabel string
}

var _ redis.Hook = (*resilienceHook)(nil)

func (h *resilienceHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h *resilienceHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		return h.guard(ctx, cmd, func(ctx context.Context) error {
			return next(ctx, cmd)
		})
	}
}

func (h *resilienceHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		var setErr = func(err error) {
			for _, cmd := range cmds {
				cmd.SetErr(err)
			}
		}
		return h.run(ctx, setErr, func(ctx context.Context) error {
			return next(ctx, cmds)
		})
	}
}

// guard runs a single command through the executor, tagging the command with a
// rejection error when the limiter or breaker short-circuits it.
func (h *resilienceHook) guard(ctx context.Context, cmd redis.Cmder, call func(context.Context) error) error {
	return h.run(ctx, cmd.SetErr, call)
}

// run is the shared body for both command and pipeline hooks. It executes call
// under the policy via [resilience.Run], treating redis.Nil (a cache miss /
// "key not found") as a success so it never trips the circuit breaker.
//
// The setErr side-channel is why go-redis keeps a thin wrapper rather than
// calling resilience.Run directly: a normal downstream failure is already
// recorded on the command(s) by go-redis itself, and for a pipeline the
// per-command errors must be preserved, not overwritten with the aggregate
// error. So setErr fires only when the command never actually ran — a
// resilience rejection or an injected fault — where go-redis had no chance to
// record anything. callErr is tracked for that distinction.
func (h *resilienceHook) run(ctx context.Context, setErr func(error), call func(context.Context) error) error {
	var callErr error
	_, err := resilience.Run(ctx, h.exec,
		func(actx context.Context) (struct{}, error) {
			callErr = call(actx)
			return struct{}{}, callErr
		}, resilience.Tolerate(redis.Nil))
	if err != nil && callErr == nil {
		setErr(err)
	}
	return err
}
