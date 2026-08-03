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
	"net"
	"sync"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/mesh"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed resolver behind each client, so
// destroyClient can stop the watch when the client is torn down.
var liveDialers sync.Map // *gorm.DB -> *discovery.Resolver

// resolverDialer adapts a discovery.Resolver to mssql's Dialer interface
// (DialContext(ctx, network, addr)). The network and addr arguments are ignored
// — the dialer picks a live endpoint via the Resolver on every call.
type resolverDialer struct {
	r  *discovery.Resolver
	nd *net.Dialer
}

func (d resolverDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	ep, err := d.r.Pick()
	if err != nil {
		return nil, err
	}
	return d.nd.DialContext(ctx, "tcp", ep.Addr)
}

var starterTag = log.RegisterInfraTag("gorm_sqlserver", "")

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
// When c.ServiceName is set (and mesh mode is off), the connection is routed
// through a Resolver that resolves the service name against the configured
// discovery backend on every dial. The mssql Connector.Dialer hook accepts our
// resolverDialer adapter, which implements mssql.Dialer. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this stays a plain DSN dial, unchanged from before.
func newClient(cp *gs.ContextProvider, c Config) (*gorm.DB, error) {
	ctx := cp.Context
	if c.Host == "" && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "gorm sqlserver: one of host or service-name must be set")
	}

	log.Debugf(ctx, starterTag, "creating gorm sqlserver client, host=%s service-name=%s db=%s", c.Host, c.ServiceName, c.DB)

	if c.ServiceName == "" || mesh.Enabled() {
		db, err := gorm.Open(sqlserver.Open(c.DSN()), gormConfig(c))
		if err != nil {
			log.Errorf(ctx, starterTag, "gorm sqlserver: open failed: %v", err)
			return nil, err
		}
		if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("microsoft.sql_server"))); err != nil {
			log.Errorf(ctx, starterTag, "gorm sqlserver: install otel plugin failed: %v", err)
			return nil, err
		}
		if err := applyPool(db, c); err != nil {
			log.Errorf(ctx, starterTag, "gorm sqlserver: ping failed: %v", err)
			if sqlDB, derr := db.DB(); derr == nil {
				_ = sqlDB.Close()
			}
			return nil, err
		}
		log.Infof(ctx, starterTag, "gorm sqlserver client initialized, host=%s db=%s", c.Host, c.DB)
		return db, nil
	}

	d, err := discovery.GetDiscovery(c.Discovery)
	if err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: get discovery backend failed: %v", err)
		return nil, err
	}
	ld, err := discovery.NewResolver(ctx, d, c.ServiceName)
	if err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: create resolver for %s failed: %v", c.ServiceName, err)
		return nil, err
	}
	msCfg, err := msdsn.Parse(c.DSN())
	if err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: parse DSN failed: %v", err)
		_ = ld.Stop()
		return nil, err
	}
	connector := mssql.NewConnectorConfig(msCfg)
	connector.Dialer = resolverDialer{r: ld, nd: &net.Dialer{}}
	sqlDB := sql.OpenDB(connector)

	db, err := gorm.Open(sqlserver.New(sqlserver.Config{Conn: sqlDB}), gormConfig(c))
	if err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: open with discovery failed: %v", err)
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("microsoft.sql_server"))); err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: install otel plugin failed: %v", err)
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	if err := applyPool(db, c); err != nil {
		log.Errorf(ctx, starterTag, "gorm sqlserver: ping failed: %v", err)
		_ = ld.Stop()
		_ = sqlDB.Close()
		return nil, err
	}
	liveDialers.Store(db, ld)
	log.Infof(ctx, starterTag, "gorm sqlserver client initialized, service-name=%s db=%s", c.ServiceName, c.DB)
	return db, nil
}

// destroyClient stops any discovery watch behind the client and closes the
// underlying connection pool.
func destroyClient(db *gorm.DB) error {
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		_ = v.(*discovery.Resolver).Stop()
		log.Debugf(context.Background(), starterTag, "gorm sqlserver client destroyed, discovery dialer stopped")
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
