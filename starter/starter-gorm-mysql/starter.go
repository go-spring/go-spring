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
	"go-spring.org/stdlib/errutil"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"
)

// liveDialers tracks the discovery-backed dialer and its registered network
// name behind each client, so destroyClient can stop the watch and deregister.
var liveDialers sync.Map // *gorm.DB -> *discoveryConn

// tlsConfigs tracks the custom TLS config name registered with the mysql driver
// for a client, so destroyClient can deregister it on teardown.
var tlsConfigs sync.Map // *gorm.DB -> string (tls config name)

// netSeq makes each registered mysql dial network name unique, so multiple
// instances discovering the same service never collide.
var netSeq atomic.Uint64

// tlsSeq makes each registered custom TLS config name unique.
var tlsSeq atomic.Uint64

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

	// Resolve the TLS DSN parameter. The shared TLS builder returns a fully
	// materialized *tls.Config when TLS is enabled (empty CAFile falls back to
	// the host's system root set; ServerName/InsecureSkipVerify honored), or
	// (nil, nil) when disabled. Register the config with the driver under a
	// unique name and reference it in the DSN as tls=<name>.
	var tlsName string
	tlsCfg, err := c.TLS.Build()
	if err != nil {
		return nil, errutil.Explain(err, "gorm-mysql: build TLS")
	}
	if tlsCfg != nil {
		tlsName = fmt.Sprintf("gstls_%d", tlsSeq.Add(1))
		if err := mysql.RegisterTLSConfig(tlsName, tlsCfg); err != nil {
			return nil, err
		}
		c.tlsParam = tlsName
	}

	dsn := c.DSN()

	var conn *discoveryConn
	if c.ServiceName != "" {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			deregisterTLS(tlsName)
			return nil, err
		}
		ld, err := discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
		if err != nil {
			deregisterTLS(tlsName)
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

	db, err := gorm.Open(gormmysql.Open(dsn), gormConfig(c))
	if err != nil {
		cleanup(conn, tlsName)
		return nil, err
	}
	if err := db.Use(tracing.NewPlugin(tracing.WithDBSystem("mysql"))); err != nil {
		cleanup(conn, tlsName)
		return nil, err
	}
	// Fail fast: verify connectivity and apply pool settings at creation time.
	if err := applyPool(db, c); err != nil {
		cleanup(conn, tlsName)
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	if err := applyResilience(c, db); err != nil {
		cleanup(conn, tlsName)
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	if conn != nil {
		liveDialers.Store(db, conn)
	}
	if tlsName != "" {
		tlsConfigs.Store(db, tlsName)
	}
	return db, nil
}

// cleanup stops a discovery dialer and deregisters driver-scoped names created
// during a failed newClient attempt.
func cleanup(conn *discoveryConn, tlsName string) {
	if conn != nil {
		_ = conn.ld.Stop()
		mysql.DeregisterDialContext(conn.netName)
	}
	deregisterTLS(tlsName)
}

// deregisterTLS removes a custom TLS config previously registered with the
// mysql driver. It is a no-op for the empty name (built-in modes).
func deregisterTLS(name string) {
	if name != "" {
		mysql.DeregisterTLSConfig(name)
	}
}

// destroyClient stops any discovery watch behind the client, deregisters its
// dial network and TLS config names, and closes the underlying connection pool.
func destroyClient(db *gorm.DB) error {
	closeResilience(db)
	if v, ok := liveDialers.LoadAndDelete(db); ok {
		conn := v.(*discoveryConn)
		_ = conn.ld.Stop()
		mysql.DeregisterDialContext(conn.netName)
	}
	if v, ok := tlsConfigs.LoadAndDelete(db); ok {
		mysql.DeregisterTLSConfig(v.(string))
	}
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
