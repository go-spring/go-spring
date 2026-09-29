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
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

func init() {
	// Register Redis clients as a group, one per entry under "${spring.go-redis}".
	//
	// Unlike a plain gs.Group, the bean type is chosen per entry from its Mode:
	// single/sentinel entries register a *redis.Client, cluster entries register
	// a *redis.ClusterClient (go-redis returns distinct types for the two). A
	// single Group cannot mix return types, so we bind the map ourselves and
	// dispatch. Switching a client to cluster is then an in-config change plus
	// swapping the injected type from *redis.Client to *redis.ClusterClient.
	gs.Module(gs.OnProperty("spring.go-redis.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.go-redis.instances}", func(name string, c Config) error {
			// Both ctors below leave index 0 (*gs.ContextProvider) to be
			// autowired and bind c explicitly. The Driver param (index 2) is
			// selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when a company provides
			// one, nil otherwise, and the ctor falls back to DefaultDriver);
			// set → that bean name, and naming a bean that does not exist
			// fails loud. The trailing governance beans (*resilience.Manager /
			// *fault.Injector, plus *loadbalance.Manager for the non-cluster
			// ctor) are injected nullable ("?"), since starter-governance may
			// legitimately be absent from the container.
			switch c.Mode {
			case "", "single", "sentinel":
				r.Provide(newClient,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.go-redis.instances."+name+".driver:=${spring.go-redis.default.driver:=?}}")),
					gs.IndexArg(3, gs.TagArg("${spring.go-redis.instances."+name+".discovery:=${spring.go-redis.default.discovery:=none}}?")),
					// The governance beans are NULLABLE injections: they exist
					// whenever starter-governance is in the container (the normal
					// case) and are absent from a container without it. Without the
					// "?" gs would treat an absent bean as a wiring error and the
					// app would not boot, turning "governance is off" into
					// "governance must be imported" — which is not the contract:
					// (*Client).Init treats a nil bean as an unarmed authority, a
					// transparent pass-through.
					gs.IndexArg(4, gs.TagArg("?")),
					gs.IndexArg(5, gs.TagArg("?")),
					gs.IndexArg(6, gs.TagArg("?")),
				).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
				// Contribute a health indicator for this instance unless the
				// user disabled it (health.enabled=false), injecting the
				// client just registered above by name.
				// Inject the concrete *Client by name and pass its
				// embedded client to the health constructor. Resolving the
				// redis.UniversalClient interface by tag would not match the
				// wrapper bean; the concrete type embeds it.
				if c.HealthEnabled {
					r.Provide(func(w *Client) *health.Indicator {
						return NewClientHealth(name, w.UniversalClient)
					}, gs.TagArg(name)).Name("redis:" + name).Caller(1)
				}
			case "cluster":
				r.Provide(newClusterClient,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.go-redis.instances."+name+".driver:=${spring.go-redis.default.driver:=?}}")),
					// The governance beans are NULLABLE injections, for the reason
					// spelled out on the non-cluster ctor above: absent without
					// starter-governance, and (*Client).Init treats a nil bean as
					// an unarmed authority, a transparent pass-through.
					gs.IndexArg(3, gs.TagArg("?")),
					gs.IndexArg(4, gs.TagArg("?")),
				).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
				if c.HealthEnabled {
					r.Provide(func(w *Client) *health.Indicator {
						return NewClusterHealth(name, w.UniversalClient)
					}, gs.TagArg(name)).Name("redis:" + name).Caller(1)
				}
			default:
				return errutil.Explain(nil, "redis: invalid mode %q for instance %q (want single/sentinel/cluster)", c.Mode, name)
			}
			// Expose this instance as a cache.Cache (the adapter lives in
			// this package's bytecache.go; both modes register a *Client, so one
			// Provide covers them). Named "go-redis:<name>" — cache.Cache is a
			// shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(c *Client) *cache.Cache {
				return cache.New(NewByteCache(c.UniversalClient))
			}, gs.TagArg(name)).Name("go-redis:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a single or sentinel Redis client, wrapped in an
// Client so Init (InitMethod) can arm resilience and the access log.
// The redisotel hooks emit client
// spans and connection-pool metrics through the OTel globals that starter-otel
// installs; when starter-otel is absent those globals are no-ops, so this stays
// a zero-config opt-in that needs no per-component adaptation.
//
// disc is the discovery backend bean cited by the entry's ${discovery} label
// (nil when the key is unset or the entry does not use service discovery).
//
// mgr, inj and lbMgr are the governance beans the container injects (all nil in
// a standalone, non-gs call). All three are retained on the Client, not handed
// to the Driver: the Driver builds a pick pool when its topology needs one and
// returns it, and Init (InitMethod) binds that pool to lbMgr. Keeping the bind
// out of the Driver is what lets a company Driver stay unaware of governance.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, disc discovery.Discovery,
	mgr *resilience.Manager, inj *fault.Injector, lbMgr *loadbalance.Manager) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating redis client, addr=%s mode=%s", c.Addr, c.Mode)

	if err := validateConfig(c); err != nil {
		return nil, err
	}
	// Fail loud when the entry routes through discovery but the cited label
	// names no backend bean — the container is the discovery directory.
	if c.ServiceName != "" && disc == nil {
		if c.Discovery == "" {
			return nil, errutil.Explain(nil, "redis: instance routes by service-name but sets no discovery backend (set ${spring.go-redis.instances.<name>.discovery} to the name of a discovery backend bean)")
		}
		return nil, errutil.Explain(nil, "redis: instance cites discovery backend %q but no such bean exists (register a discovery backend bean under that name)", c.Discovery)
	}
	// When service discovery owns the address, a configured addr can never take
	// effect — say so instead of dropping it silently.
	if c.ServiceName != "" && c.Addr != "" {
		log.Warnf(ctx.Context, log.TagAppDef, "redis: addr %q is ignored for instance with service-name %q: the address is resolved via service discovery", c.Addr, c.ServiceName)
	}
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, lbPool, err := d.CreateClient(ctx.Context, c, disc)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: create client failed: %v", err)
		return nil, err
	}
	w := &Client{UniversalClient: client, cfg: c, mgr: mgr, inj: inj, lbMgr: lbMgr, lbPool: lbPool}
	if err := instrument(client, c.Otel); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: instrument client failed: %v", err)
		_ = w.Close()
		return nil, err
	}
	if err := failFastPing(ctx.Context, c, client); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: startup ping failed: %v", err)
		_ = w.Close()
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "redis client initialized, addr=%s mode=%s", c.Addr, c.Mode)
	return w, nil
}

// newClusterClient creates a cluster Redis client, wrapped in an
// Client. The driver must implement ClusterDriver; the redisotel
// hooks attach per-node via ClusterClient.OnNewNode, so tracing/metrics cover
// every node discovered.
//
// mgr and inj are the governance beans the container injects (both nil in a
// standalone, non-gs call); they are retained on the Client for Init
// (InitMethod) to arm the per-command executor with. Cluster mode self-discovers
// its nodes, so no endpoint-selection pool exists and no manager is threaded to
// the driver.
func newClusterClient(ctx *gs.ContextProvider, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating redis cluster client, addrs=%v", c.Addrs)

	if err := validateConfig(c); err != nil {
		return nil, err
	}
	// No company Driver bean → fall back to the bundled default assembly (which
	// implements ClusterDriver).
	if d == nil {
		d = DefaultDriver{}
	}
	cd, ok := d.(ClusterDriver)
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: the configured Driver does not support cluster mode (implement ClusterDriver)")
		return nil, errutil.Explain(nil, "redis: the configured Driver does not support cluster mode (implement ClusterDriver)")
	}
	client, err := cd.CreateClusterClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: create cluster client failed: %v", err)
		return nil, err
	}
	w := &Client{UniversalClient: client, cfg: c, mgr: mgr, inj: inj}
	if err := instrument(client, c.Otel); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: instrument cluster client failed: %v", err)
		_ = w.Close()
		return nil, err
	}
	if err := failFastPing(ctx.Context, c, client); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redis: cluster startup ping failed: %v", err)
		_ = w.Close()
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "redis cluster client initialized, addrs=%v", c.Addrs)
	return w, nil
}

// validateConfig checks the per-mode required fields, and rejects combining
// service discovery with sentinel/cluster (which self-discover their nodes).
func validateConfig(c Config) error {
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

// instrument attaches redisotel tracing and metrics per otel, accepting any
// topology via redis.UniversalClient (*redis.Client and *redis.ClusterClient
// both satisfy it). Each signal is gated by its flag, so an instance can opt out
// of one (e.g. per-command spans) while keeping the other.
func instrument(client redis.UniversalClient, otel OtelConfig) error {
	if otel.TracingEnabled {
		if err := redisotel.InstrumentTracing(client); err != nil {
			return err
		}
	}
	if otel.MetricsEnabled {
		if err := redisotel.InstrumentMetrics(client); err != nil {
			return err
		}
	}
	return nil
}

// failFastPing verifies the connection is usable at startup so a misconfigured
// address or unreachable server surfaces during boot rather than on the first
// request. It applies to all three topologies. The DialTimeout bounds the probe.
func failFastPing(ctx context.Context, c Config, client redis.UniversalClient) error {
	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout(c))
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return errutil.Explain(err, "redis: startup ping failed")
	}
	return nil
}

// pingTimeout picks a bound for the startup ping: the configured DialTimeout
// when set, otherwise a conservative default.
func pingTimeout(c Config) time.Duration {
	if c.DialTimeout > 0 {
		return c.DialTimeout
	}
	return 5 * time.Second
}

// Client teardown is handled by (*Client).Destroy (the gs destroy
// method): it closes the resilience executor (if armed), releases the
// endpoint-selection subscription (when armed), and closes the underlying
// client.
