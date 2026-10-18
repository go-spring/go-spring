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

// Package StarterGormClickhouse is the gorm+clickhouse dialect starter. It
// registers one gorm client per entry under "spring.gorm.clickhouse" — the
// gs.Module block below is that registration, written out here so a reader sees
// the beans this starter contributes — with the shared open/pool/observe/
// resilience scaffolding provided by go-spring.org/starter-gorm. The
// ClickHouse-specific pieces — the Config + DSN, TLS and the service-discovery
// dialer — live here.
package StarterGormClickhouse

import (
	"context"
	"net"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/mesh"
	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/label"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	gormcore "go-spring.org/starter-gorm"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"gorm.io/driver/clickhouse"
)

// The registration below is written out per dialect starter rather than shared
// through a helper, so a reader of this file sees exactly which beans it
// contributes: one *DB named "clickhouse.<entry>" plus a paired health.Indicator,
// per entry under spring.gorm.clickhouse.instances. The construction those beans
// run — build → [gormcore.NewDB] (open, observe, governance, startup ping) — is
// shared in starter-gorm.
func init() {
	gs.Module(gs.OnProperty("spring.gorm.clickhouse.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an entry does not define falls back to the family-wide
		// "default" bucket: spring.gorm.clickhouse.default.<k> is the value every
		// entry inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.gorm.clickhouse.instances", "spring.gorm.clickhouse.default")
		return conf.BindEach(p, "${spring.gorm.clickhouse.instances}", func(name string, c Config) error {
			// The dialect qualifier keeps clickhouse's instances in their own
			// bean-name space, so two dialects may carry an instance of the same
			// name.
			beanName := "clickhouse." + name
			r.Provide(func(ctx *gs.ContextProvider, discoveryLabel string, center *runtime.Runtime) (*gormcore.DB, error) {
				// The wiring tag applies the family-default discovery fallback
				// (discoveryLabel); fold it into the config so the dialect's build
				// reads one label — c.Discovery — and resolves the backend itself,
				// failing construction if it names no bean. "none" is the tag's
				// sentinel for "no backend".
				if discoveryLabel != "" && discoveryLabel != "none" {
					c.Discovery = discoveryLabel
				}
				spec, err := build(ctx.Context, c, center)
				if err != nil {
					return nil, err
				}
				return gormcore.NewDB(ctx.Context, "clickhouse", spec, center, c.Ping)
			},
				gs.IndexArg(1, gs.TagArg("${spring.gorm.clickhouse.instances."+name+".discovery:=${spring.gorm.clickhouse.default.discovery:=none}}")),
			).Name(beanName).Destroy((*gormcore.DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the bean just registered above
			// by name.
			if c.Health {
				r.Provide(func(w *gormcore.DB) *health.Indicator {
					return gormcore.NewClientHealth("gorm:clickhouse:", name, w)
				}, gs.TagArg(beanName)).Name("gorm:clickhouse:" + name).Caller(1)
			}
			return nil
		})
	})
}

// build constructs the driver-specific dialector for a Config, handling TLS and
// service discovery, and returns the Spec [gormcore.NewDB] assembles.
//
// When c.ServiceName is set (and mesh mode is off), the connection is routed
// through a Resolver: the ClickHouse native driver builds a *sql.DB with our
// DialContext, so each new connection reaches a live instance resolved from the
// discovery backend and address changes take effect without rebuilding the
// client. In mesh mode a sidecar owns discovery+LB, so the configured Addr is
// used as-is. When c.ServiceName is empty this is a plain DSN dial, unchanged
// from before.
func build(ctx context.Context, c Config, center *runtime.Runtime) (gormcore.Spec, error) {
	if c.Addr == "" && c.ServiceName == "" {
		return gormcore.Spec{}, errutil.Explain(nil, "gorm clickhouse: one of addr or service-name must be set")
	}

	log.Debug(ctx, log.TagAppDef, "creating gorm clickhouse client", func() []log.Field {
		return []log.Field{log.String("addr", c.Addr), log.String("service_name", c.ServiceName), log.String("db", c.DB)}
	})

	service := label.ServiceLabel("gorm:clickhouse", c.ServiceName, c.Addr)

	// Resolve the discovery backend the entry cites before building the dialer:
	// the build reads one label (c.Discovery) and fails construction when it
	// names no bean, rather than silently dialing the static address.
	var backend discovery.Discovery
	if c.ServiceName != "" {
		backend, _ = center.Discovery().Get(c.Discovery)
		if backend == nil {
			return gormcore.Spec{}, errutil.Explain(nil, "gorm clickhouse: instance routes by service-name but cites discovery backend %q, which names no bean", c.Discovery)
		}
	}

	var (
		dialector = clickhouse.Open(c.DSN())
	)

	// The native driver (ch.OpenDB) is required whenever we must inject a custom
	// TLS config or a discovery-backed dialer, neither of which the URL-style DSN
	// can express. Otherwise the plain DSN path stays as before. Mesh mode skips
	// discovery (sidecar owns it) but may still need native for TLS.
	useDiscovery := c.ServiceName != "" && !mesh.Enabled()
	useNative := useDiscovery || c.TLS.Enabled
	if useNative {
		opts := &ch.Options{
			Addr: []string{c.Addr},
			Auth: ch.Auth{
				Database: c.DB,
				Username: c.User,
				Password: c.Password,
			},
			DialTimeout: c.DialTimeout,
			ReadTimeout: c.ReadTimeout,
		}
		if c.TLS.Enabled {
			tlsCfg, terr := c.TLS.BuildClient()
			if terr != nil {
				log.Error(ctx, log.TagAppDef, terr, "gorm clickhouse build TLS failed")
				return gormcore.Spec{}, errutil.Explain(terr, "gorm-clickhouse: build TLS")
			}
			opts.TLS = tlsCfg
		}
		if useDiscovery {
			lb, _, derr := c.NewPickPool(ctx, backend, service, center.Loadbalance())
			if derr != nil {
				log.Error(ctx, log.TagAppDef, derr, "gorm clickhouse build discovery resolver failed")
				return gormcore.Spec{}, derr
			}
			// ch.Options.DialContext is 2-arg: func(ctx, addr string) (net.Conn, error).
			// The addr is ignored; the dialer picks a live endpoint from the
			// chain's current generation.
			nd := &net.Dialer{}
			opts.DialContext = func(ctx context.Context, _ string) (net.Conn, error) {
				ep, perr := lb.Pick(loadbalance.PickInfo{})
				if perr != nil {
					return nil, perr
				}
				conn, derr := nd.DialContext(ctx, "tcp", ep.Addr)
				// The dial outcome is the only signal this picker has; feeding it
				// makes outlier suspension evict an instance that keeps refusing
				// connections.
				lb.Complete(ctx, ep, derr)
				return conn, derr
			}
		}
		dialector = clickhouse.New(clickhouse.Config{Conn: ch.OpenDB(opts)})
	}

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Service:        service,
		ObserveEnabled: c.ObserveEnabled,
	}, nil
}

// Discovery dialer — a client can be dialed straight from a configured Addr, or
// (when ServiceName is set and mesh mode is off) through a discovery resolver
// whose background watch keeps the endpoint set fresh. The resolver (built by
// [gormcore.Common.NewPickPool]) is adapted to the native driver's DialContext,
// and its watch is stopped via the closer [build] attaches to the client.
