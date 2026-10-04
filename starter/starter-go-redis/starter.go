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

package StarterGoRedis

import (
	"context"
	"go-spring.org/cloud/governance"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register Redis clients as a group, one per entry under "${spring.go-redis}".
	//
	// Unlike a plain gs.Group, the bean type is chosen per entry from its Mode:
	// single/sentinel entries register a *Client over a *redis.Client, cluster
	// entries register a *Client over a *redis.ClusterClient (go-redis returns
	// distinct raw types for the two; the wrapper hides that difference, so both
	// register the same *Client bean type). A single Group cannot mix return
	// types, so we bind the map ourselves and dispatch. Switching a client to
	// cluster is then a pure in-config change.
	gs.Module(gs.OnProperty("spring.go-redis.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.go-redis.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.go-redis.instances", "spring.go-redis.default")
		return conf.BindEach(p, "${spring.go-redis.instances}", func(name string, c Config) error {
			// Both ctors below leave index 0 (*gs.ContextProvider) to be
			// autowired and bind c explicitly. The Driver param (index 2) is
			// selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when a company provides
			// one, nil otherwise, and the ctor falls back to DefaultDriver);
			// set → that bean name, and naming a bean that does not exist
			// fails loud.
			//
			// There is no Init hook: the ctor assembles the client completely
			// (identity + observation + governance + selection binding) before it
			// returns, so gs only has to know how to DESTROY it.
			switch c.Mode {
			case "", "single", "sentinel":
				r.Provide(newClient,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.go-redis.instances."+name+".driver:=${spring.go-redis.default.driver:=?}}")),
					gs.IndexArg(3, gs.TagArg("${spring.go-redis.instances."+name+".discovery:=${spring.go-redis.default.discovery:=none}}")),
					// The governance center is the family's sole injection point: it hands
					// out the resilience/fault/loadbalance authorities.
				).Name(name).Destroy((*Client).Destroy).Caller(1)
				// Contribute a health indicator for this instance unless the
				// user disabled it (health=false), injecting the
				// client just registered above by name. The probe goes to the
				// wrapper, which holds the raw client.
				if c.Health {
					r.Provide(func(w *Client) *health.Indicator {
						return NewClientHealth(name, w)
					}, gs.TagArg(name)).Name("redis:" + name).Caller(1)
				}
			case "cluster":
				r.Provide(newClusterClient,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.go-redis.instances."+name+".driver:=${spring.go-redis.default.driver:=?}}")),
					// The governance center is the family's sole injection point: it hands
					// out the resilience/fault/loadbalance authorities.
				).Name(name).Destroy((*Client).Destroy).Caller(1)
				if c.Health {
					r.Provide(func(w *Client) *health.Indicator {
						return NewClientHealth(name, w)
					}, gs.TagArg(name)).Name("redis:" + name).Caller(1)
				}
			default:
				return errutil.Explain(nil, "redis: invalid mode %q for instance %q (want single/sentinel/cluster)", c.Mode, name)
			}
			// Expose this instance as a cache.Cache (the adapter lives in
			// this package's bytecache.go; both modes register a *Client, so one
			// Provide covers them). The wrapper IS a redis.UniversalClient, so
			// the adapter takes it directly. Named "go-redis:<name>" — cache.Cache
			// is a shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(c *Client) *cache.Cache {
				return cache.New(NewByteCache(c))
			}, gs.TagArg(name)).Name("go-redis:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a single or sentinel Redis client, wrapped in a [Client].
// The redisotel hooks emit connection-pool metrics through the OTel globals
// that starter-otel installs; when starter-otel is absent those globals are
// no-ops, so this stays a zero-config opt-in that needs no per-component
// adaptation. The per-command span and access log come from the resilience
// layer, which reads the identity this starter declares.
//
// disc is the backend the entry's ${discovery} label resolves to in the center's
// discovery directory (nil when the key is unset, sentinel "none", or names
// nothing, and the entry does not use service discovery).
//
// center is the governance center the container injects (nil in a standalone,
// non-gs call) — the family's sole injection point. Its resilience, fault and
// loadbalance authorities are bundled into a [cloud.ClientParams] and
// handed to the Driver — not applied to the Driver's product afterwards: the
// Driver passes the bundle to [NewClient], which applies the resilience executor
// and binds the pool the Driver built to the loadbalance authority. The client is therefore
// assembled complete in one step, with the zero bundle degrading to an
// observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, discoveryLabel string,
	center *governance.Center) (*Client, error) {
	// The client's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context,
		log.String("addr", c.Addr),
		log.String("mode", c.Mode))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{log.Msg("creating redis client")}
	})

	if err := validateConfig(ctx.Context, c); err != nil {
		return nil, err
	}

	disc, _ := center.Discovery().Get(discoveryLabel)

	// Fail loud when the entry routes through discovery but the cited label
	// names no backend — the center's discovery directory is the table.
	if c.ServiceName != "" && disc == nil {
		if c.Discovery == "" {
			return nil, errutil.Explain(nil, "redis: instance routes by service-name but sets no discovery backend (set ${spring.go-redis.instances.<name>.discovery} to the name of a discovery backend bean)")
		}
		return nil, errutil.Explain(nil, "redis: instance cites discovery backend %q but no such bean exists (register a discovery backend bean under that name)", c.Discovery)
	}
	// When service discovery owns the address, a configured addr can never take
	// effect — say so instead of dropping it silently.
	if c.ServiceName != "" && c.Addr != "" {
		log.Warn(cctx, log.TagAppDef,
			log.String("service_name", c.ServiceName),
			log.Msg("redis: addr is ignored for an instance with a service-name: the address is resolved via service discovery"))
	}
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	// The Driver returns the client complete — identity, observation and
	// governance all applied while it was built. There is no Init hook and
	// nothing runs after this: the bean is finished when the ctor returns.
	w, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault(), Loadbalance: center.Loadbalance(), Discovery: disc})
	if err != nil {
		log.Error(cctx, log.TagAppDef, err, log.Msg("redis: create client failed"))
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	// Fail fast (opt-in, e.g. ping=true): ping the backend at startup so a
	// misconfigured address or an unreachable server surfaces during boot rather
	// than on the first request. The probe is [HealthCheck] — the same single
	// health implementation the Actuator indicator uses — and the hooks are
	// already on the raw client the wrapper embeds, which is exactly what
	// "assemble first, then probe" means. A failure abandons the client, so
	// release everything just assembled. With ping unset the probe is skipped
	// and an unreachable server only surfaces on first use.
	if c.Ping {
		pingCtx, cancel := context.WithTimeout(ctx.Context, pingTimeout(c))
		err = HealthCheck(pingCtx, w)
		cancel()
		if err != nil {
			log.Error(cctx, log.TagAppDef, err, log.Msg("redis: startup ping failed"))
			_ = w.Destroy()
			return nil, errutil.Explain(err, "redis: startup ping failed")
		}
	}
	log.Info(cctx, log.TagAppDef, log.Msg("create redis client success"))
	return w, nil
}

// newClusterClient creates a cluster Redis client, wrapped in a [Client]. The
// driver must implement [ClusterDriver]; the redisotel metric hooks attach
// per-node via ClusterClient.OnNewNode, so the pool metrics cover every node
// discovered.
//
// center is the governance center the container injects (nil in a standalone,
// non-gs call) — the family's sole injection point; its resilience and fault
// authorities are bundled into a [cloud.ClientParams] and
// handed to the Driver, which builds the per-command executor while assembling
// the client. Cluster mode self-discovers its nodes, so no endpoint-selection
// pool exists and the bundle carries no loadbalance authority.
func newClusterClient(ctx *gs.ContextProvider, c Config, d Driver, center *governance.Center) (*Client, error) {
	// See newClient: the cluster's node list rides on a context derived here.
	cctx := log.WithFields(ctx.Context, log.Strings("addrs", c.Addrs))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{log.Msg("creating redis cluster client")}
	})

	if err := validateConfig(ctx.Context, c); err != nil {
		return nil, err
	}
	// No company Driver bean → fall back to the bundled default assembly (which
	// implements ClusterDriver).
	if d == nil {
		d = DefaultDriver{}
	}
	cd, ok := d.(ClusterDriver)
	if !ok {
		err := errutil.Explain(nil, "redis: the configured Driver does not support cluster mode (implement ClusterDriver)")
		log.Error(cctx, log.TagAppDef, err,
			log.Msg("redis: the configured Driver does not support cluster mode (implement ClusterDriver)"))
		return nil, err
	}
	w, err := cd.CreateClusterClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		log.Error(cctx, log.TagAppDef, err, log.Msg("redis: create cluster client failed"))
		return nil, errutil.Explain(err, "failed to create redis cluster client")
	}
	if c.Ping {
		pingCtx, cancel := context.WithTimeout(ctx.Context, pingTimeout(c))
		err = HealthCheck(pingCtx, w)
		cancel()
		if err != nil {
			log.Error(cctx, log.TagAppDef, err, log.Msg("redis: startup cluster ping failed"))
			_ = w.Destroy()
			return nil, errutil.Explain(err, "redis: startup ping failed")
		}
	}
	log.Info(cctx, log.TagAppDef, log.Msg("create redis cluster client success"))
	return w, nil
}

// validateConfig checks the per-mode required fields, and rejects combining
// service discovery with sentinel/cluster (which self-discover their nodes).
// It also reports a configuration key this starter has removed.
func validateConfig(ctx context.Context, c Config) error {
	if c.Otel.TracingEnabled {
		// A removed key must not fail the bind — that would turn the framework's
		// own change into a failed deploy — but it must not pass in silence
		// either: the behaviour it used to switch is gone for every instance.
		log.Warn(ctx, log.TagAppDef,
			log.Msg("redis: otel.tracing.enabled is removed and ignored — the per-command span is emitted by the resilience layer; remove the key from your configuration"))
	}
	switch c.Mode {
	case "", "single":
		if err := errutil.RequireAny("redis",
			errutil.Field{Name: "addr", Value: c.Addr},
			errutil.Field{Name: "service-name", Value: c.ServiceName},
		); err != nil {
			return err
		}
	case "sentinel":
		if c.ServiceName != "" {
			return errutil.Explain(nil, "redis: service-name is not supported in sentinel mode")
		}
		if c.MasterName == "" || len(c.SentinelAddrs) == 0 {
			return errutil.Explain(nil, "redis: master-name and sentinel-addrs are required in sentinel mode")
		}
	case "cluster":
		if c.ServiceName != "" {
			return errutil.Explain(nil, "redis: service-name is not supported in cluster mode")
		}
		if len(c.Addrs) == 0 {
			return errutil.Explain(nil, "redis: addrs is required in cluster mode")
		}
		// Redis Cluster exposes no databases (only db 0 exists), so a non-zero
		// db cannot take effect; fail fast instead of silently dropping it.
		if c.DB != 0 {
			return errutil.Explain(nil, "redis: db is not supported in cluster mode (redis cluster has no database select), got db=%d", c.DB)
		}
	default:
		return errutil.Explain(nil, "redis: invalid mode %q (want single/sentinel/cluster)", c.Mode)
	}
	return nil
}

// instrument attaches redisotel's metrics per otel, accepting any topology via
// redis.UniversalClient (*redis.Client and *redis.ClusterClient both satisfy
// it). Only the connection-POOL metrics remain: they are a non-per-call
// observable-gauge family, so they stay here, while the per-command span this
// used to attach is gone — the resilience layer opens the one call span by
// itself. The signal is gated by its flag, so an instance can opt out. It is
// called by [NewClient] before the declaration hook is added, so the redisotel
// hooks stay outermost in the command chain.
func instrument(client redis.UniversalClient, otel OtelConfig) error {
	if otel.MetricsEnabled {
		if err := redisotel.InstrumentMetrics(client); err != nil {
			return err
		}
	}
	return nil
}

// pingTimeout picks a bound for the startup probe: the configured DialTimeout
// when set, otherwise a conservative default.
func pingTimeout(c Config) time.Duration {
	if c.DialTimeout > 0 {
		return c.DialTimeout
	}
	return 5 * time.Second
}
