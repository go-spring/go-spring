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
// registers one gorm client per entry under "spring.gorm.clickhouse", with the
// shared open/pool/observe/resilience scaffolding provided by
// go-spring.org/starter-gorm. Only the ClickHouse-specific pieces — the Config +
// DSN, TLS and the service-discovery dialer — live here.
package StarterGormClickhouse

import (
	"context"
	"net"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/mesh"
	"go-spring.org/log"
	gormcore "go-spring.org/starter-gorm"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/clickhouse"
)

func init() {
	gormcore.Module(gormcore.Dialect[Config]{
		Prefix:       "spring.gorm.clickhouse",
		BeanPrefix:   "clickhouse",
		Engine:       "clickhouse",
		HealthPrefix: "gorm:clickhouse:",
		Build:        build,
	})
}

// build constructs the driver-specific dialector for a Config, handling TLS and
// service discovery, and returns the Spec gormcore.Module needs to open and
// wrap the client.
//
// When c.ServiceName is set (and mesh mode is off), the connection is routed
// through a Resolver: the ClickHouse native driver builds a *sql.DB with our
// DialContext, so each new connection reaches a live instance resolved from the
// discovery backend and address changes take effect without rebuilding the
// client. In mesh mode a sidecar owns discovery+LB, so the configured Addr is
// used as-is. When c.ServiceName is empty this is a plain DSN dial, unchanged
// from before.
func build(ctx context.Context, c Config, backend discovery.Discovery) (gormcore.Spec, error) {
	if c.Addr == "" && c.ServiceName == "" {
		return gormcore.Spec{}, errutil.Explain(nil, "gorm clickhouse: one of addr or service-name must be set")
	}

	log.Debugf(ctx, log.TagAppDef, "creating gorm clickhouse client, addr=%s service-name=%s db=%s", c.Addr, c.ServiceName, c.DB)

	var (
		dialector = clickhouse.Open(c.DSN())
		closer    func()
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
				log.Errorf(ctx, log.TagAppDef, "gorm clickhouse: build TLS failed: %v", terr)
				return gormcore.Spec{}, errutil.Explain(terr, "gorm-clickhouse: build TLS")
			}
			opts.TLS = tlsCfg
		}
		if useDiscovery {
			lb, _, derr := c.NewPickPool(ctx, backend)
			if derr != nil {
				log.Errorf(ctx, log.TagAppDef, "gorm clickhouse: build discovery resolver failed: %v", derr)
				return gormcore.Spec{}, derr
			}
			// ch.Options.DialContext is 2-arg: func(ctx, addr string) (net.Conn, error).
			// The addr is ignored; the dialer picks a live endpoint via the Resolver.
			nd := &net.Dialer{}
			opts.DialContext = func(ctx context.Context, _ string) (net.Conn, error) {
				ep, perr := lb.Pick(loadbalance.PickInfo{})
				if perr != nil {
					return nil, perr
				}
				return nd.DialContext(ctx, "tcp", ep.Addr)
			}
			closer = nil
		}
		dialector = clickhouse.New(clickhouse.Config{Conn: ch.OpenDB(opts)})
	}

	closers := []func(){}
	if closer != nil {
		closers = append(closers, closer)
	}

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Resource:       resilience.ResourceLabel("gorm:clickhouse", c.ServiceName, c.Addr),
		ObserveEnabled: c.ObserveEnabled,
		Closers:        closers,
	}, nil
}

// Discovery dialer — a client can be dialed straight from a configured Addr, or
// (when ServiceName is set and mesh mode is off) through a discovery resolver
// whose background watch keeps the endpoint set fresh. The resolver (built by
// [gormcore.Common.NewPickPool]) is adapted to the native driver's DialContext,
// and its watch is stopped via the closer [build] attaches to the client.
