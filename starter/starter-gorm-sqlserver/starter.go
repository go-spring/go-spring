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
// registers one gorm client per entry under "spring.gorm.sqlserver", with the
// shared open/pool/observe/resilience scaffolding provided by
// go-spring.org/starter-gorm. Only the SQL Server-specific pieces — the Config +
// DSN and the service-discovery dialer — live here.
package StarterGormSqlserver

import (
	"context"
	"database/sql"
	"net"

	mssql "github.com/microsoft/go-mssqldb"
	"github.com/microsoft/go-mssqldb/msdsn"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	gormcore "go-spring.org/starter-gorm"
	"go-spring.org/stdlib/errutil"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

func init() {
	gormcore.Module(gormcore.Dialect[Config]{
		Prefix:       "spring.gorm.sqlserver",
		BeanPrefix:   "sqlserver",
		Engine:       "microsoft.sql_server",
		HealthPrefix: "gorm:sqlserver:",
		Build:        build,
	})
}

// build constructs the driver-specific dialector for a Config, handling service
// discovery, and returns the Spec gormcore.Module needs to open and wrap the
// client.
//
// When c.ServiceName is set (and mesh mode is off), the connection is routed
// through a Resolver that resolves the service name against the configured
// discovery backend on every dial. The mssql Connector.Dialer hook accepts our
// resolverDialer adapter, which implements mssql.Dialer. In mesh mode a sidecar
// owns discovery+LB, so the configured Host is used as-is. When c.ServiceName
// is empty this stays a plain DSN dial, unchanged from before.
func build(ctx context.Context, c Config, backend discovery.Discovery) (gormcore.Spec, error) {
	if c.Host == "" && c.ServiceName == "" {
		return gormcore.Spec{}, errutil.Explain(nil, "gorm sqlserver: one of host or service-name must be set")
	}

	log.Debugf(ctx, log.TagAppDef, "creating gorm sqlserver client, host=%s service-name=%s db=%s", c.Host, c.ServiceName, c.DB)

	var (
		dialector gorm.Dialector
		closer    func()
	)

	lb, _, err := c.NewPickPool(ctx, backend)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gorm sqlserver: build discovery resolver failed: %v", err)
		return gormcore.Spec{}, err
	}
	if lb != nil {
		msCfg, err := msdsn.Parse(c.DSN())
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "gorm sqlserver: parse DSN failed: %v", err)
			return gormcore.Spec{}, err
		}
		connector := mssql.NewConnectorConfig(msCfg)
		connector.Dialer = resolverDialer{lb: lb, nd: &net.Dialer{}}
		dialector = sqlserver.New(sqlserver.Config{Conn: sql.OpenDB(connector)})
	} else {
		dialector = sqlserver.Open(c.DSN())
	}

	closers := []func(){}
	if closer != nil {
		closers = append(closers, closer)
	}

	return gormcore.Spec{
		Dialector:      dialector,
		Pool:           c.Pool(),
		Resource:       resilience.ResourceLabel("gorm:sqlserver", c.ServiceName, c.Host),
		ObserveEnabled: c.ObserveEnabled,
		Closers:        closers,
	}, nil
}

// Discovery dialer — the live-resolver lifecycle concept. A client can be dialed
// straight from a configured Host, or (when ServiceName is set and mesh mode is
// off) through a discovery resolver whose background watch keeps the endpoint
// set fresh. The resolver is adapted to mssql's Dialer interface via
// resolverDialer, and its watch is stopped via the closer [build] attaches to
// the client.

// resolverDialer adapts the shared round-robin pick pool to mssql's Dialer
// (DialContext(ctx, network, addr)). The network and addr arguments are ignored
// — the dialer picks a live endpoint via the Resolver on every call.
type resolverDialer struct {
	lb *loadbalance.Pool
	nd *net.Dialer
}

func (d resolverDialer) DialContext(ctx context.Context, _, _ string) (net.Conn, error) {
	ep, err := d.lb.Pick(loadbalance.PickInfo{})
	if err != nil {
		return nil, err
	}
	return d.nd.DialContext(ctx, "tcp", ep.Addr)
}

// The pool (built by [gormcore.Common.NewPickPool]) is adapted to mssql's
// Dialer interface via resolverDialer, and its watch is stopped via the closer
// [build] attaches to the client.
