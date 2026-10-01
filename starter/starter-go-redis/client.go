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
// Destroy) and service label. It mirrors starter-redigo's pool.go: the entity
// embeds the raw client and carries the resilience executor + the
// endpoint-selection subscription, while the per-command hook layers (the
// declaration layer in observe.go and the executor in command.go) live beside
// it (starter-redigo's conn.go analog).
package StarterGoRedis

import (
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud"
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
	exec resilience.ClientExecutor
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
