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

package StarterGormPostgres

import (
	"context"
	"net"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/mesh"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed resolver behind each client, so
// destroyClient can stop the background watch when the client is torn down.
var liveDialers sync.Map // *gorm.DB -> *discovery.Resolver

var starterTag = log.RegisterInfraTag("gorm_postgres", "")

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.postgres}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.postgres}", newClient, destroyClient)
}

// newClient creates a GORM database client using the PostgreSQL driver, bridged
// into go-spring's unified observability. The otel plugin emits client spans and
// connection-pool metrics through the OTel globals that starter-otel installs;
// when starter-otel is absent those globals are no-ops, so this stays a
// zero-config, zero-overhead opt-in that needs no per-component adaptation.
//
// When c.ServiceName is set (and mesh mode is off), the address is resolved
// through the registered discovery backend: a Resolver is bound to the pgx
// DialFunc so each new physical connection reaches a live instance and address
// changes take effect without rebuilding the client. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this is a plain DSN dial, unchanged from before.
func newClient(cp *gs.ContextProvider, c Config) (*gorm.DB, error) {
	ctx := cp.Context
	if c.Host == "" && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "gorm postgres: one of host or service-name must be set")
	}

	log.Debugf(ctx, starterTag, "creating gorm postgres client, host=%s service-name=%s db=%s", c.Host, c.ServiceName, c.DB)

	var (
		db  *gorm.DB
		err error
		ld  *discovery.Resolver
	)

	if c.ServiceName == "" || mesh.Enabled() {
		db, err = gorm.Open(postgres.Open(c.DSN()), gormConfig(c))
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm postgres: open failed: %v", err)
			return nil, err
		}
	} else {
		d, err := discovery.GetDiscovery(c.Discovery)
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm postgres: get discovery backend failed: %v", err)
			return nil, err
		}
		ld, err = discovery.NewResolver(ctx, d, c.ServiceName, discovery.WithScheme(c.Scheme))
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm postgres: create resolver for %s failed: %v", c.ServiceName, err)
			return nil, err
		}
		pgxCfg, err := pgx.ParseConfig(c.DSN())
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm postgres: parse pgx config failed: %v", err)
			_ = ld.Stop()
			return nil, err
		}
		// pgconn.DialFunc is 3-arg: func(ctx, network, addr string) (net.Conn, error).
		// Both network and addr are ignored; the dialer picks a live endpoint via
		// the Resolver and dials it over TCP.
		nd := &net.Dialer{}
		pgxCfg.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
			ep, perr := ld.Pick()
			if perr != nil {
				return nil, perr
			}
			return nd.DialContext(ctx, "tcp", ep.Addr)
		}
		sqlDB := stdlib.OpenDB(*pgxCfg)
		db, err = gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), gormConfig(c))
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm postgres: open with discovery failed: %v", err)
			_ = sqlDB.Close()
			_ = ld.Stop()
			return nil, err
		}
	}

	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("postgresql"))); err != nil {
		log.Errorf(ctx, starterTag, "gorm postgres: install otel plugin failed: %v", err)
		if ld != nil {
			_ = ld.Stop()
		}
		return nil, err
	}
	// Fail fast: verify connectivity and apply pool settings at creation time.
	if err := applyPool(db, c); err != nil {
		log.Errorf(ctx, starterTag, "gorm postgres: ping failed: %v", err)
		if ld != nil {
			_ = ld.Stop()
		}
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	if ld != nil {
		liveDialers.Store(db, ld)
	}
	log.Infof(ctx, starterTag, "gorm postgres client initialized, host=%s db=%s", c.Host, c.DB)
	return db, nil
}

// destroyClient stops any discovery watch behind the client and closes the
// underlying connection pool.
func destroyClient(db *gorm.DB) error {
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		_ = v.(*discovery.Resolver).Stop()
		log.Debugf(context.Background(), starterTag, "gorm postgres client destroyed, discovery dialer stopped")
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
