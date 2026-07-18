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
	"go-spring.org/stdlib/discovery"
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
	if c.ServiceName == "" {
		db, err = gorm.Open(clickhouse.Open(c.DSN()))
		if err != nil {
			return nil, err
		}
	} else {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			return nil, err
		}
		ld, err = discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
		if err != nil {
			return nil, err
		}
		sqlDB := ch.OpenDB(&ch.Options{
			Addr: []string{c.Addr},
			Auth: ch.Auth{
				Database: c.DB,
				Username: c.User,
				Password: c.Password,
			},
			DialTimeout: c.DialTimeout,
			ReadTimeout: c.ReadTimeout,
			DialContext: ld.Dial,
		})
		db, err = gorm.Open(clickhouse.New(clickhouse.Config{Conn: sqlDB}))
		if err != nil {
			_ = ld.Stop()
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
