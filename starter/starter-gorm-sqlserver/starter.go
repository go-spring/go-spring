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

package StarterGormSqlserver

import (
	"context"
	"database/sql"
	"sync"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed dialer behind each client, so
// destroyClient can stop the watch when the client is torn down.
var liveDialers sync.Map // *gorm.DB -> *discovery.LiveDialer

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.sqlserver}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.sqlserver}", newClient, destroyClient)
}

// newClient creates a GORM database client using the SQL Server driver, bridged
// into go-spring's unified observability. The otel plugin emits client spans and
// connection-pool metrics through the OTel globals that starter-otel installs;
// when starter-otel is absent those globals are no-ops, so this stays a
// zero-config, zero-overhead opt-in that needs no per-component adaptation.
//
// When c.ServiceName is set, the connection is routed through a LiveDialer that
// resolves the service name against the configured discovery backend on every
// dial. The mssql Connector.Dialer hook accepts our LiveDialer directly since
// its DialContext signature matches mssql.Dialer. When c.ServiceName is empty
// this stays a plain DSN dial, unchanged from before.
func newClient(c Config) (*gorm.DB, error) {
	if c.Host == "" && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "gorm sqlserver: one of host or service-name must be set")
	}
	if c.ServiceName == "" {
		db, err := gorm.Open(sqlserver.Open(c.DSN()), gormConfig(c))
		if err != nil {
			return nil, err
		}
		if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("microsoft.sql_server"))); err != nil {
			return nil, err
		}
		if err := applyPool(db, c); err != nil {
			if sqlDB, derr := db.DB(); derr == nil {
				_ = sqlDB.Close()
			}
			return nil, err
		}
		return db, nil
	}

	d, err := discovery.MustGet(c.Discovery)
	if err != nil {
		return nil, err
	}
	ld, err := discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
	if err != nil {
		return nil, err
	}
	msCfg, err := msdsn.Parse(c.DSN())
	if err != nil {
		_ = ld.Stop()
		return nil, err
	}
	connector := mssql.NewConnectorConfig(msCfg)
	connector.Dialer = ld
	sqlDB := sql.OpenDB(connector)

	db, err := gorm.Open(sqlserver.New(sqlserver.Config{Conn: sqlDB}), gormConfig(c))
	if err != nil {
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("microsoft.sql_server"))); err != nil {
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	if err := applyPool(db, c); err != nil {
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	liveDialers.Store(db, ld)
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
