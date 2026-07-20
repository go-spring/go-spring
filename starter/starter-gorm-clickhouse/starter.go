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

package StarterGormClickhouse

import (
	"context"
	"sync"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/discovery"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/clickhouse"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed dialer behind each client, so
// destroyClient can stop the watch when the client is torn down.
var liveDialers sync.Map // *gorm.DB -> *discovery.LiveDialer

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.clickhouse}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.clickhouse}", newClient, destroyClient)
}

// newClient creates a GORM database client using the ClickHouse driver, bridged
// into go-spring's unified observability. The otel plugin emits client spans and
// connection-pool metrics through the OTel globals that starter-otel installs;
// when starter-otel is absent those globals are no-ops, so this stays a
// zero-config, zero-overhead opt-in that needs no per-component adaptation.
//
// When c.ServiceName is set, the connection is routed through a LiveDialer:
// the ClickHouse native driver builds a *sql.DB with our DialContext, so each
// new connection reaches a live instance resolved from the discovery backend
// and address changes take effect without rebuilding the client. When
// c.ServiceName is empty this is a plain DSN dial, unchanged from before.
func newClient(c Config) (*gorm.DB, error) {
	if c.Addr == "" && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "gorm clickhouse: one of addr or service-name must be set")
	}
	var (
		db  *gorm.DB
		err error
		ld  *discovery.LiveDialer
	)

	// The native driver (ch.OpenDB) is required whenever we must inject a custom
	// TLS config or a discovery-backed dialer, neither of which the URL-style DSN
	// can express. Otherwise the plain DSN path stays as before.
	useNative := c.ServiceName != "" || c.TLS.Enabled
	if !useNative {
		db, err = gorm.Open(clickhouse.Open(c.DSN()), gormConfig(c))
		if err != nil {
			return nil, err
		}
	} else {
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
			tlsCfg, terr := c.TLS.Build()
			if terr != nil {
				return nil, errutil.Explain(terr, "gorm-clickhouse: build TLS")
			}
			opts.TLS = tlsCfg
		}
		if c.ServiceName != "" {
			d, derr := discovery.MustGet(c.Discovery)
			if derr != nil {
				return nil, derr
			}
			ld, derr = discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
			if derr != nil {
				return nil, derr
			}
			opts.DialContext = ld.Dial
		}
		sqlDB := ch.OpenDB(opts)
		db, err = gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB}), gormConfig(c))
		if err != nil {
			if ld != nil {
				_ = ld.Stop()
			}
			_ = sqlDB.Close()
			return nil, err
		}
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("clickhouse"))); err != nil {
		if ld != nil {
			_ = ld.Stop()
		}
		return nil, err
	}
	// Fail fast: verify connectivity and apply pool settings at creation time.
	if err := applyPool(db, c); err != nil {
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
	return db, nil
}

// destroyClient stops any discovery watch behind the client and closes the
// underlying connection pool.
func destroyClient(db *gorm.DB) error {
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		_ = v.(*discovery.LiveDialer).Stop()
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
