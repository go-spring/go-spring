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

// Package StarterGormMySql is the gorm+mysql dialect starter. It registers one
// gorm client per entry under "spring.gorm.mysql", with the shared open/pool/
// observe/resilience scaffolding provided by go-spring.org/starter-gorm. Only
// the MySQL-specific pieces — the Config + DSN, TLS registration and the
// service-discovery dialer — live here.
package StarterGormMySql

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/go-sql-driver/mysql"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/starter-gorm"
	"go-spring.org/stdlib/errutil"
	gormmysql "gorm.io/driver/mysql"
)

// tlsSeq makes each registered custom TLS config name unique.
var tlsSeq atomic.Uint64

func init() {
	gormcore.Module(gormcore.Dialect[Config]{
		Prefix:       "spring.gorm.mysql",
		BeanPrefix:   "mysql",
		Engine:       "mysql",
		HealthPrefix: "gorm:mysql:",
		Build:        build,
	})
}

// build constructs the driver-specific dialector for a Config, handling TLS and
// service-discovery routing, and returns the Spec gormcore.Module needs to
// open and wrap the client.
//
// When c.ServiceName is set (and mesh mode is off), the address is resolved
// through the registered discovery backend: a Resolver is bound to a unique
// mysql dial network name and the DSN routes through it, so each new connection
// reaches a live instance and address changes take effect without rebuilding
// the client. In mesh mode a sidecar owns discovery+LB, so the configured Addr
// is used as-is. When c.ServiceName is empty this is a plain Addr dial,
// unchanged from before.
func build(ctx context.Context, c Config, backend discovery.Discovery) (gormcore.Spec, error) {
	if c.Addr == "" && c.ServiceName == "" {
		return gormcore.Spec{}, fmt.Errorf("gorm mysql: one of addr or service-name must be set")
	}

	log.Debugf(ctx, log.TagAppDef, "creating gorm mysql client, addr=%s service-name=%s db=%s", c.Addr, c.ServiceName, c.DB)

	var (
		discoCloser func()
		tlsCloser   func()
	)

	// Resolve the TLS DSN parameter. The shared TLS builder returns a fully
	// materialized *tls.Config when TLS is enabled (empty CAFile falls back to
	// the host's system root set; ServerName/InsecureSkipVerify honored), or
	// (nil, nil) when disabled. Register the config with the driver under a
	// unique name and reference it in the DSN as tls=<name>.
	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gorm mysql: build TLS failed: %v", err)
		return gormcore.Spec{}, errutil.Explain(err, "gorm-mysql: build TLS")
	}
	if tlsCfg != nil {
		tlsName := fmt.Sprintf("gstls_%d", tlsSeq.Add(1))
		if err := mysql.RegisterTLSConfig(tlsName, tlsCfg); err != nil {
			return gormcore.Spec{}, err
		}
		c.tlsParam = tlsName
		tlsCloser = func() { mysql.DeregisterTLSConfig(tlsName) }
	}

	dsn := c.DSN()

	resource := resilience.ResourceLabel("gorm:mysql", c.ServiceName, c.Addr)
	conn, err := newDiscoveryConn(ctx, c, backend, resource)
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "gorm mysql: build discovery dialer failed: %v", err)
		if tlsCloser != nil {
			tlsCloser()
		}
		return gormcore.Spec{}, err
	}
	if conn != nil {
		// Route the DSN through the registered dialer; Addr becomes a label the
		// dialer ignores since it picks a live endpoint itself.
		dc := c
		dc.Network = conn.netName
		dc.Addr = c.ServiceName
		dsn = dc.DSN()
		discoCloser = func() { stopDiscoveryConn(conn) }
	}

	closers := []func(){}
	if discoCloser != nil {
		closers = append(closers, discoCloser)
	}
	if tlsCloser != nil {
		closers = append(closers, tlsCloser)
	}

	return gormcore.Spec{
		Dialector:      gormmysql.Open(dsn),
		Pool:           c.Pool(),
		Resource:       resource,
		ObserveEnabled: c.ObserveEnabled,
		Closers:        closers,
	}, nil
}

// Discovery dialer — the live-dialer + resolver lifecycle concept. A client can
// be dialed straight from a configured Addr, or (when ServiceName is set and
// mesh mode is off) through a discovery resolver whose background watch keeps
// the endpoint set fresh. The resolver is registered under a unique mysql dial
// network name, and its watch is stopped (and the dialer deregistered) via the
// closer [build] attaches to the client.

// netSeq makes each registered mysql dial network name unique, so multiple
// instances discovering the same service never collide.
var netSeq atomic.Uint64

// discoveryConn pairs a live Resolver with the unique mysql dial network name
// it registered, so the closer can stop the watch and deregister the dialer.
type discoveryConn struct {
	netName string
	stop    func() // detaches the pool's endpoint-selection binding
}

// newDiscoveryConn resolves the registered discovery backend for c and registers
// a mysql dialer that routes each new connection through a live endpoint. It
// returns (nil, nil) when service-name is unset or mesh mode is enabled (a
// sidecar owns discovery+LB), in which case the caller dials the configured Addr
// directly. The caller owns the lifecycle and must release the conn via
// stopDiscoveryConn. resource is the entry's governance label, to which the
// pool's endpoint selection is bound.
func newDiscoveryConn(ctx context.Context, c Config, backend discovery.Discovery, resource string) (*discoveryConn, error) {
	lb, _, stop, err := c.NewPickPool(ctx, backend, resource)
	if err != nil {
		return nil, err
	}
	if lb == nil {
		return nil, nil
	}
	netName := fmt.Sprintf("gsdisco_%s_%d", c.ServiceName, netSeq.Add(1))
	nd := &net.Dialer{}
	// mysql.DialContextFunc is 2-arg: func(ctx, addr string) (net.Conn, error).
	// The addr is ignored; the dialer picks a live endpoint via the pool.
	mysql.RegisterDialContext(netName, func(ctx context.Context, _ string) (net.Conn, error) {
		ep, perr := lb.Pick(loadbalance.PickInfo{})
		if perr != nil {
			return nil, perr
		}
		conn, derr := nd.DialContext(ctx, "tcp", ep.Addr)
		// The dial outcome is the only signal this picker has; feeding it makes
		// outlier suspension evict an instance that keeps refusing connections.
		lb.Complete(ep, derr)
		return conn, derr
	})
	return &discoveryConn{netName: netName, stop: stop}, nil
}

// stopDiscoveryConn stops the discovery watch and deregisters the mysql dialer
// behind conn. It is the close-half of the discovery lifecycle, symmetric with
// newDiscoveryConn; it is a no-op for a nil conn (a client that never had one).
func stopDiscoveryConn(conn *discoveryConn) {
	if conn == nil {
		return
	}
	mysql.DeregisterDialContext(conn.netName)
	conn.stop()
}
