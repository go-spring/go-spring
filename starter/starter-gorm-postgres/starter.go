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
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed dialer behind each client, so
// destroyClient can stop the background watch when the client is torn down.
var liveDialers sync.Map // *gorm.DB -> *discovery.LiveDialer

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
// When c.ServiceName is set, the address is resolved through the registered
// discovery backend: a LiveDialer is bound to the pgx DialFunc so each new
// physical connection reaches a live instance and address changes take effect
// without rebuilding the client. When c.ServiceName is empty this is a plain
// DSN dial, unchanged from before.
func newClient(c Config) (*gorm.DB, error) {
	if c.Host == "" && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "gorm postgres: one of host or service-name must be set")
	}

	log.Debugf(context.Background(), starterTag, "creating gorm postgres client, host=%s service-name=%s db=%s", c.Host, c.ServiceName, c.DB)

	var (
		db  *gorm.DB
		err error
		ld  *discovery.LiveDialer
	)

	if c.ServiceName == "" {
		db, err = gorm.Open(postgres.Open(c.DSN()), gormConfig(c))
		if err != nil {
			log.Errorf(context.Background(), starterTag, "gorm postgres: open failed: %v", err)
			return nil, err
		}
	} else {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			log.Errorf(context.Background(), starterTag, "gorm postgres: get discovery backend failed: %v", err)
			return nil, err
		}
		ld, err = discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
		if err != nil {
			log.Errorf(context.Background(), starterTag, "gorm postgres: create live dialer for %s failed: %v", c.ServiceName, err)
			return nil, err
		}
		pgxCfg, err := pgx.ParseConfig(c.DSN())
		if err != nil {
			log.Errorf(context.Background(), starterTag, "gorm postgres: parse pgx config failed: %v", err)
			_ = ld.Stop()
			return nil, err
		}
		// LiveDialer.DialContext matches pgconn.DialFunc exactly:
		// func(ctx context.Context, network, addr string) (net.Conn, error).
		// It ignores the addr and connects to a live instance.
		pgxCfg.DialFunc = ld.DialContext
		sqlDB := stdlib.OpenDB(*pgxCfg)
		db, err = gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), gormConfig(c))
		if err != nil {
			log.Errorf(context.Background(), starterTag, "gorm postgres: open with discovery failed: %v", err)
			_ = sqlDB.Close()
			_ = ld.Stop()
			return nil, err
		}
	}

	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("postgresql"))); err != nil {
		log.Errorf(context.Background(), starterTag, "gorm postgres: install otel plugin failed: %v", err)
		if ld != nil {
			_ = ld.Stop()
		}
		return nil, err
	}
	// Fail fast: verify connectivity and apply pool settings at creation time.
	if err := applyPool(db, c); err != nil {
		log.Errorf(context.Background(), starterTag, "gorm postgres: ping failed: %v", err)
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
	log.Infof(context.Background(), starterTag, "gorm postgres client initialized, host=%s db=%s", c.Host, c.DB)
	return db, nil
}

// destroyClient stops any discovery watch behind the client and closes the
// underlying connection pool.
func destroyClient(db *gorm.DB) error {
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		_ = v.(*discovery.LiveDialer).Stop()
		log.Debugf(context.Background(), starterTag, "gorm postgres client destroyed, discovery dialer stopped")
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
