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

package StarterRegistryZookeeper

import (
	"context"
	"errors"
	"strings"

	"github.com/go-zookeeper/zk"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// zkCenter owns the ONE shared *zk.Conn for the ensemble configured under
// ${spring.registry.zookeeper}. Both halves of the naming idiom derive from it:
// the registrar (the write side, starter.go) and the discovery backend
// auto-derived for the same ensemble (below). Constructing the connection once
// — instead of one session per side — is the "one center config, two beans"
// shape: a dual-role application configures the ensemble a single time.
//
// The bean's Destroy closes the connection, so neither derived side closes it.
type zkCenter struct {
	conn   *zk.Conn
	config ZookeeperConfig
}

// connectZookeeper dials the ensemble for c, applies digest auth when set, and
// probes it (an Exists call blocks until the session connects) so an
// unreachable ensemble fails startup rather than surfacing on the first
// operation. It is the single construction path behind the center, the
// registrar fallback, and standalone discovery blocks.
func connectZookeeper(c ZookeeperConfig) (*zk.Conn, error) {
	if len(c.Servers) == 0 {
		return nil, errutil.Explain(nil, "registry-zookeeper: servers is required")
	}
	conn, _, err := zk.Connect(c.Servers, c.SessionTimeout)
	if err != nil {
		log.Errorf(context.Background(), log.TagAppDef, "connect zookeeper servers=%v failed: %v", c.Servers, err)
		return nil, errutil.Explain(err, "registry-zookeeper: connect to %v", c.Servers)
	}
	if c.Username != "" || c.Password != "" {
		if err := conn.AddAuth("digest", []byte(c.Username+":"+c.Password)); err != nil {
			conn.Close()
			return nil, errutil.Explain(err, "registry-zookeeper: add digest auth")
		}
	}
	// Fail-fast probe: an Exists call blocks until the session connects (or the
	// session timeout elapses), so an unreachable ensemble surfaces at boot.
	if _, _, err := conn.Exists("/"); err != nil {
		conn.Close()
		return nil, errutil.Explain(err, "registry-zookeeper: startup probe failed for %v", c.Servers)
	}
	return conn, nil
}

// newZkCenter builds the shared connection and probes the ensemble. The probe
// is the fail-fast both sides relied on when they built their own connections:
// a misconfigured or unreachable ensemble fails startup here, once.
func newZkCenter(c ZookeeperConfig) (*zkCenter, error) {
	conn, err := connectZookeeper(c)
	if err != nil {
		return nil, err
	}
	return &zkCenter{conn: conn, config: c}, nil
}

// Close releases the shared connection. It is the bean destructor.
func (z *zkCenter) Close() error {
	if z == nil || z.conn == nil {
		return nil
	}
	z.conn.Close()
	return nil
}

func init() {
	// The center bean exists exactly when the ensemble is configured. It is
	// nullable-injected (TagArg("?")) by the registrar Server and by discovery
	// backends that inherit the center connection, so it is constructed only
	// when at least one side consumes it; its destructor closes the connection
	// once.
	//
	// When discovery-name is non-empty (the default "zookeeper"), the module
	// also derives a discovery backend bean for the same ensemble under that
	// label — the "one config block serves both halves" default. A dual-role
	// app then cites discovery=zookeeper without any
	// ${spring.discovery.zookeeper} block; a label colliding with another bean
	// name fails loudly in the container.
	gs.Module(gs.OnProperty("spring.registry.zookeeper.servers"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c ZookeeperConfig
		if err := conf.Bind(p, &c, "${spring.registry.zookeeper}"); err != nil {
			return errutil.Explain(err, "registry-zookeeper: bind center config")
		}
		r.Provide(newZkCenter,
			gs.IndexArg(0, gs.ValueArg(c)),
		).Destroy((*zkCenter).Close).Caller(1)

		if name := c.DiscoveryName; name != "" {
			r.Provide(newCenterDiscoveryBackend,
				gs.IndexArg(0, gs.TagArg("?")),
				gs.IndexArg(1, gs.ValueArg(strings.TrimRight(c.BasePath, "/"))),
			).Name(name).Caller(1)
		}
		return nil
	})
}

// newCenterDiscoveryBackend serves snapshots for the center's ensemble through
// the shared connection. It is the discovery bean auto-derived from
// ${spring.registry.zookeeper}; base-path is the center's (the registrar writes
// under it), so read and write can never diverge.
func newCenterDiscoveryBackend(zc *zkCenter, basePath string) (discovery.Discovery, error) {
	if zc == nil {
		return nil, errutil.Explain(nil, "registry-zookeeper: center connection unavailable")
	}
	log.Debugf(context.Background(), log.TagAppDef, "derived zookeeper discovery backend from center config, servers=%v", zc.config.Servers)
	return &zkDiscovery{
		conn:     zc.conn,
		basePath: basePath,
		done:     make(chan struct{}),
		entries:  map[string]*serviceEntry{},
	}, nil
}

// errConnectionClosed reports whether err means the zk connection is closed
// for good (no retry can succeed).
func errConnectionClosed(err error) bool {
	return errors.Is(err, zk.ErrConnectionClosed) || errors.Is(err, zk.ErrClosing)
}
