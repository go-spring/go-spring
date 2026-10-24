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

// Package StarterGormPostgres is the gorm+postgres dialect starter. It registers
// one gorm client per entry under "spring.gorm.postgres" — the gs.Module block
// below is that registration, written out here so a reader sees the beans this
// starter contributes — with the shared open/pool/observe/resilience scaffolding
// provided by go-spring.org/starter-gorm. The PostgreSQL-specific pieces — the
// Config + DSN and the service-discovery dialer — live here.
package StarterGormPostgres

import (
	"context"
	"net"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go-spring.org/cloud/actuator/health"
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
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// The registration below is written out per dialect starter rather than shared
// through a helper, so a reader of this file sees exactly which beans it
// contributes: one *DB named "postgres.<entry>" plus a paired health.Indicator,
// per entry under spring.gorm.postgres.instances. The construction those beans
// run — build → [gormcore.NewDB] (open, observe, governance, startup ping) — is
// shared in starter-gorm.
func init() {
	gs.Module(gs.OnProperty("spring.gorm.postgres.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an entry does not define falls back to the family-wide
		// "default" bucket: spring.gorm.postgres.default.<k> is the value every
		// entry inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.gorm.postgres.instances", "spring.gorm.postgres.default")
		return conf.BindEach(p, "${spring.gorm.postgres.instances}", func(name string, c Config) error {
			// The dialect qualifier keeps postgres's instances in their own
			// bean-name space, so two dialects may carry an instance of the same
			// name.
			beanName := "postgres." + name
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
				return gormcore.NewDB(ctx.Context, "postgresql", spec, center, c.Ping)
			},
				gs.IndexArg(1, gs.TagArg("${spring.gorm.postgres.instances."+name+".discovery:=${spring.gorm.postgres.default.discovery:=none}}")),
			).Name(beanName).Destroy((*gormcore.DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the bean just registered above
			// by name.
			if c.Health {
				r.Provide(func(w *gormcore.DB) *health.Indicator {
					return gormcore.NewClientHealth("gorm:postgres:", name, w)
				}, gs.TagArg(beanName)).Name("gorm:postgres:" + name).Caller(1)
			}
			return nil
		})
	})
}

// build constructs the driver-specific dialector for a Config, handling service
// discovery, and returns the Spec [gormcore.NewDB] assembles.
// client.
//
// When c.ServiceName is set (and mesh mode is off), the address is resolved
// through the registered discovery backend: a Resolver is bound to the pgx
// DialFunc so each new physical connection reaches a live instance and address
// changes take effect without rebuilding the client. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this is a plain DSN dial, unchanged from before.
func build(ctx context.Context, c Config, center *runtime.Runtime) (gormcore.Spec, error) {
	if c.Host == "" && c.ServiceName == "" {
		return gormcore.Spec{}, errutil.Explain(nil, "gorm postgres: one of host or service-name must be set")
	}

	// TLS.* and sslmode must agree: sslmode still decides whether TLS is
	// negotiated, so tls.enabled against sslmode=disable would silently dial
	// plaintext. Fail loudly at startup instead.
	if c.TLS.Enabled && c.SSLMode == "disable" {
		return gormcore.Spec{}, errutil.Explain(nil,
			"gorm postgres: tls.enabled=true conflicts with sslmode=disable; set sslmode to require (or a verifying mode) or turn tls off")
	}

	log.Debug(ctx, log.TagAppDef, "creating gorm postgres client", func() []log.Field {
		return []log.Field{log.String("host", c.Host), log.String("service_name", c.ServiceName), log.String("db", c.DB)}
	})

	service := label.ServiceLabel("gorm:postgresql", c.ServiceName, c.Host)

	// Resolve the discovery backend the entry cites before building the dialer:
	// the build reads one label (c.Discovery) and fails construction when it
	// names no bean, rather than silently dialing the static host.
	var backend discovery.Discovery
	if c.ServiceName != "" {
		backend, _ = center.Discovery().Get(c.Discovery)
		if backend == nil {
			return gormcore.Spec{}, errutil.Explain(nil, "gorm postgres: instance routes by service-name but cites discovery backend %q, which names no bean", c.Discovery)
		}
	}

	var dialector gorm.Dialector

	lb, ld, err := c.NewPickPool(ctx, backend, service, center.Loadbalance())
	if err != nil {
		log.Error(ctx, log.TagAppDef, err, "gorm postgres build discovery resolver failed")
		return gormcore.Spec{}, err
	}

	// Always go through a parsed pgx config: it validates the DSN once at
	// startup and gives one injection point for TLS and the discovery dialer.
	pgxCfg, err := pgx.ParseConfig(c.DSN())
	if err != nil {
		log.Error(ctx, log.TagAppDef, err, "gorm postgres parse pgx config failed")
		return gormcore.Spec{}, err
	}
	if c.TLS.Enabled {
		tlsCfg, terr := c.TLS.BuildClient()
		if terr != nil {
			log.Error(ctx, log.TagAppDef, terr, "gorm postgres build TLS failed")
			return gormcore.Spec{}, errutil.Explain(terr, "gorm postgres: build TLS")
		}
		pgxCfg.TLSConfig = tlsCfg
	}
	if ld != nil {
		// pgconn.DialFunc is 3-arg: func(ctx, network, addr string) (net.Conn, error).
		// Both network and addr are ignored; the dialer picks a live endpoint via
		// the Resolver and dials it over TCP.
		nd := &net.Dialer{}
		pgxCfg.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
			ep, perr := lb.Pick(ctx, loadbalance.PickInfo{})
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
	dialector = postgres.New(postgres.Config{Conn: stdlib.OpenDB(*pgxCfg)})

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Service:        service,
		ObserveEnabled: c.ObserveEnabled,
	}, nil
}

// Discovery dialer — a client can be dialed straight from a configured Host, or
// (when ServiceName is set and mesh mode is off) through a discovery resolver
// whose background watch keeps the endpoint set fresh. The resolver (built by
// [gormcore.Common.NewPickPool]) is adapted to pgx's DialFunc
// stopped via the closer [build] attaches to the client.
