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

package StarterGormMySql

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/discovery"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed dialer and its registered network
// name behind each client, so destroyClient can stop the watch and deregister.
var liveDialers sync.Map // *gorm.DB -> *discoveryConn

// netSeq makes each registered mysql dial network name unique, so multiple
// instances discovering the same service never collide.
var netSeq atomic.Uint64

type discoveryConn struct {
	ld      *discovery.LiveDialer
	netName string
}

func init() {
	// Register multiple GORM clients as a group.
	// Each instance is created according to the configuration in "${spring.gorm.mysql}".
	// This allows defining multiple database connections dynamically.
	gs.Group("${spring.gorm.mysql}", newClient, destroyClient)
}

// newClient creates a GORM database client using the MySQL driver, bridged into
// go-spring's unified observability. The otel plugin emits client spans and
// connection-pool metrics through the OTel globals that starter-otel installs;
// when starter-otel is absent those globals are no-ops, so this stays a
// zero-config, zero-overhead opt-in that needs no per-component adaptation.
//
// When c.ServiceName is set, the address is resolved through the registered
// discovery backend: a LiveDialer is bound to a unique mysql dial network name
// and the DSN routes through it, so each new connection reaches a live instance
// and address changes take effect without rebuilding the client. When
// c.ServiceName is empty this is a plain Addr dial, unchanged from before.
func newClient(c Config) (*gorm.DB, error) {
	if c.Addr == "" && c.ServiceName == "" {
		return nil, fmt.Errorf("gorm mysql: one of addr or service-name must be set")
	}
	dsn := c.DSN()

	var conn *discoveryConn
	if c.ServiceName != "" {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			return nil, err
		}
		ld, err := discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
		if err != nil {
			return nil, err
		}
		netName := fmt.Sprintf("gsdisco_%s_%d", c.ServiceName, netSeq.Add(1))
		mysql.RegisterDialContext(netName, ld.Dial)

		// Route the DSN through the registered dialer; Addr becomes a label the
		// dialer ignores since it picks a live endpoint itself.
		dc := c
		dc.Network = netName
		dc.Addr = c.ServiceName
		dsn = dc.DSN()
		conn = &discoveryConn{ld: ld, netName: netName}
	}

	db, err := gorm.Open(gormmysql.Open(dsn))
	if err != nil {
		if conn != nil {
			_ = conn.ld.Stop()
			mysql.DeregisterDialContext(conn.netName)
		}
		return nil, err
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("mysql"))); err != nil {
		return nil, err
	}
	if conn != nil {
		liveDialers.Store(db, conn)
	}
	return db, nil
}

// destroyClient stops any discovery watch behind the client, deregisters its
// dial network name, and closes the underlying connection pool.
func destroyClient(db *gorm.DB) error {
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		conn := v.(*discoveryConn)
		_ = conn.ld.Stop()
		mysql.DeregisterDialContext(conn.netName)
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
