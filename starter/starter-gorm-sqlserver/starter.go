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

// Package StarterGormSqlserver is the gorm+sqlserver dialect starter. It
// registers one gorm client per entry under "spring.gorm.sqlserver" — the
// gs.Module block below is that registration, written out here so a reader sees
// the beans this starter contributes — with the shared open/pool/observe/
// resilience scaffolding provided by go-spring.org/starter-gorm. The SQL
// Server-specific pieces — the Config + DSN and the service-discovery dialer —
// live here.
package StarterGormSqlserver

import (
	"context"
	"database/sql"
	"net"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
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
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// The registration below is written out per dialect starter rather than shared
// through a helper, so a reader of this file sees exactly which beans it
// contributes: one *DB named "sqlserver.<entry>" plus a paired health.Indicator,
// per entry under spring.gorm.sqlserver.instances. The construction those beans
// run — build → [gormcore.NewDB] (open, observe, governance, startup ping) — is
// shared in starter-gorm.
func init() {
	gs.Module(gs.OnProperty("spring.gorm.sqlserver.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an entry does not define falls back to the family-wide
		// "default" bucket: spring.gorm.sqlserver.default.<k> is the value every
		// entry inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.gorm.sqlserver.instances", "spring.gorm.sqlserver.default")
		return conf.BindEach(p, "${spring.gorm.sqlserver.instances}", func(name string, c Config) error {
			// The dialect qualifier keeps sqlserver's instances in their own
			// bean-name space, so two dialects may carry an instance of the same
			// name.
			beanName := "sqlserver." + name
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
				return gormcore.NewDB(ctx.Context, "microsoft.sql_server", spec, center, c.Ping)
			},
				gs.IndexArg(1, gs.TagArg("${spring.gorm.sqlserver.instances."+name+".discovery:=${spring.gorm.sqlserver.default.discovery:=none}}")),
			).Name(beanName).Destroy((*gormcore.DB).Destroy).Caller(1)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the bean just registered above
			// by name.
			if c.Health {
				r.Provide(func(w *gormcore.DB) *health.Indicator {
					return gormcore.NewClientHealth("gorm:sqlserver:", name, w)
				}, gs.TagArg(beanName)).Name("gorm:sqlserver:" + name).Caller(1)
			}
			return nil
		})
	})
}

// build constructs the driver-specific dialector for a Config, handling service
// discovery, and returns the Spec [gormcore.NewDB] assembles.
//
// When c.ServiceName is set (and mesh mode is off), the connection is routed
// through a Resolver that resolves the service name against the configured
// discovery backend on every dial. The mssql Connector.Dialer hook accepts our
// resolverDialer adapter, which implements mssql.Dialer. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this stays a plain DSN dial, unchanged from before.
func build(ctx context.Context, c Config, center *runtime.Runtime) (gormcore.Spec, error) {
	if c.Host == "" && c.ServiceName == "" {
		return gormcore.Spec{}, errutil.Explain(nil, "gorm sqlserver: one of host or service-name must be set")
	}

	log.Debug(ctx, log.TagAppDef, "creating gorm sqlserver client", func() []log.Field {
		return []log.Field{log.String("host", c.Host), log.String("service_name", c.ServiceName), log.String("db", c.DB)}
	})

	service := label.ServiceLabel("gorm:sqlserver", c.ServiceName, c.Host)

	// Resolve the discovery backend the entry cites before building the dialer:
	// the build reads one label (c.Discovery) and fails construction when it
	// names no bean, rather than silently dialing the static host.
	var backend discovery.Discovery
	if c.ServiceName != "" {
		backend, _ = center.Discovery().Get(c.Discovery)
		if backend == nil {
			return gormcore.Spec{}, errutil.Explain(nil, "gorm sqlserver: instance routes by service-name but cites discovery backend %q, which names no bean", c.Discovery)
		}
	}

	var dialector gorm.Dialector

	lb, _, err := c.NewPickPool(ctx, backend, service, center.Loadbalance())
	if err != nil {
		log.Error(ctx, log.TagAppDef, err, "gorm sqlserver build discovery resolver failed")
		return gormcore.Spec{}, err
	}
	if lb != nil {
		msCfg, err := msdsn.Parse(c.DSN())
		if err != nil {
			log.Error(ctx, log.TagAppDef, err, "gorm sqlserver parse DSN failed")
			return gormcore.Spec{}, err
		}
		connector := mssql.NewConnectorConfig(msCfg)
		connector.Dialer = resolverDialer{lb: lb, nd: &net.Dialer{}}
		dialector = sqlserver.New(sqlserver.Config{Conn: sql.OpenDB(connector)})
	} else {
		dialector = sqlserver.Open(c.DSN())
	}

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Service:        service,
		ObserveEnabled: c.ObserveEnabled,
	}, nil
}

// Discovery dialer — the live-resolver lifecycle concept. A client can be dialed
// straight from a configured Host, or (when ServiceName is set and mesh mode is
// off) through a discovery resolver whose background watch keeps the endpoint
// set fresh. The resolver is adapted to mssql's Dialer interface via
// resolverDialer, and its watch is stopped via the closer [build] attaches to
// the client.

// resolverDialer adapts the shared endpoint-selection chain to mssql's Dialer
// (DialContext(ctx, network, addr)). The network and addr arguments are ignored
// — the dialer picks a live endpoint from the chain's current generation on
// every call.
type resolverDialer struct {
	lb *loadbalance.Chain
	nd *net.Dialer
}

func (d resolverDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	ep, err := d.lb.Pick(ctx, loadbalance.PickInfo{})
	if err != nil {
		return nil, err
	}
	conn, derr := d.nd.DialContext(ctx, "tcp", ep.Addr)
	// The dial outcome is the only signal this picker has; feeding it makes
	// outlier suspension evict an instance that keeps refusing connections.
	d.lb.Complete(ctx, ep, derr)
	return conn, derr
}

// The chain (built by [gormcore.Common.NewPickPool]) is adapted to mssql's
// Dialer interface via resolverDialer, and its watch is stopped via the closer
// [build] attaches to the client.
