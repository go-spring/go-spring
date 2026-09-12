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
// one gorm client per entry under "spring.gorm.postgres", with the shared open/
// pool/observe/resilience scaffolding provided by go-spring.org/starter-gorm.
// Only the PostgreSQL-specific pieces — the Config + DSN and the service-
// discovery dialer — live here.
package StarterGormPostgres

import (
	"context"
	"net"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	gormcore "go-spring.org/starter-gorm"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func init() {
	gormcore.Module(gormcore.Dialect[Config]{
		Prefix:       "spring.gorm.postgres",
		BeanPrefix:   "postgres",
		Engine:       "postgresql",
		HealthPrefix: "gorm:postgres:",
		Build:        build,
	})
}

// build constructs the driver-specific dialector for a Config, handling service
// discovery, and returns the Spec gormcore.Module needs to open and wrap the
// client.
//
// When c.ServiceName is set (and mesh mode is off), the address is resolved
// through the registered discovery backend: a Resolver is bound to the pgx
// DialFunc so each new physical connection reaches a live instance and address
// changes take effect without rebuilding the client. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this is a plain DSN dial, unchanged from before.
func build(ctx context.Context, c Config, backend discovery.Discovery) (gormcore.Spec, error) {
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

	log.Debugf(ctx, log.TagAppDef, "creating gorm postgres client, host=%s service-name=%s db=%s", c.Host, c.ServiceName, c.DB)

	var (
		dialector gorm.Dialector
		closer    func()
	)

	lb, ld, err := c.NewPickPool(ctx, backend)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gorm postgres: build discovery resolver failed: %v", err)
		return gormcore.Spec{}, err
	}

	// Always go through a parsed pgx config: it validates the DSN once at
	// startup and gives one injection point for TLS and the discovery dialer.
	pgxCfg, err := pgx.ParseConfig(c.DSN())
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gorm postgres: parse pgx config failed: %v", err)
		return gormcore.Spec{}, err
	}
	if c.TLS.Enabled {
		tlsCfg, terr := c.TLS.BuildClient()
		if terr != nil {
			log.Errorf(ctx, log.TagAppDef, "gorm postgres: build TLS failed: %v", terr)
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
			ep, perr := lb.Pick(loadbalance.PickInfo{})
			if perr != nil {
				return nil, perr
			}
			return nd.DialContext(ctx, "tcp", ep.Addr)
		}
		closer = nil
	}
	dialector = postgres.New(postgres.Config{Conn: stdlib.OpenDB(*pgxCfg)})

	closers := []func(){}
	if closer != nil {
		closers = append(closers, closer)
	}

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Resource:       resilience.ResourceLabel("gorm:postgresql", c.ServiceName, c.Host),
		ObserveEnabled: c.ObserveEnabled,
		Closers:        closers,
	}, nil
}

// Discovery dialer — a client can be dialed straight from a configured Host, or
// (when ServiceName is set and mesh mode is off) through a discovery resolver
// whose background watch keeps the endpoint set fresh. The resolver (built by
// [gormcore.Common.NewPickPool]) is adapted to pgx's DialFunc
// stopped via the closer [build] attaches to the client.
