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

// Package StarterDiscoveryZookeeper adapts ZooKeeper as a service registry.
// Each ${spring.discovery.zookeeper.<name>} block becomes ONE backend bean
// named "zookeeper.<name>" serving both sides of the naming idiom: the write
// side (a discovery.Registry collected by the discovery registration core, which
// registers this instance once the app is ready and deregisters it on
// shutdown) and the read side (a discovery.Discovery consumers cite by the
// bean's name). Blank-import the package and configure one block per
// ensemble:
//
//	spring.discovery.zookeeper.main.servers=127.0.0.1:2181
//	spring.discovery.service-name=orders
//	spring.discovery.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-discovery-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to ZooKeeper.
//
// Each instance is written as an ephemeral znode. An ephemeral node lives only
// as long as the client session, so if the process dies without deregistering,
// ZooKeeper removes the node once the session expires - self-healing without a
// reaper.
//
// The registration core - the single discoveryServer bean that publishes into
// every configured center (across backends) - lives in cloud/discovery, which
// this package imports anyway, so it exists exactly once per process.
package StarterDiscoveryZookeeper

import (
	"context"
	"strings"

	"github.com/go-zookeeper/zk"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute, so one dashboard can compare discovery centers.
const obsSystem = "zookeeper"

var (
	// starterTag identifies logs emitted by the zookeeper discovery starter.
	starterTag = log.RegisterAppTag("discovery_zookeeper", "")
)

// zkBackend is ONE configured discovery center: the
// ${spring.discovery.zookeeper.<name>} block made a bean. It owns the single
// ZooKeeper session for that ensemble (probed at construction, closed by the
// bean destructor) and serves both halves of the naming idiom through it: the
// write side (a discovery.Registry collected by the discovery registration core)
// and the read side (a discovery.Discovery consumers cite by the bean's name
// "zookeeper.<name>"; lazy, so an app that never cites it pays nothing for
// the read half). Both sides share the block's base-path, so read and write
// can never diverge.
type zkBackend struct {
	reg  *zkRegistry
	disc *zkDiscovery

	// obs is this block's observability layer: one observer per configured
	// block, closed by the bean destructor below.
	obs *discovery.Observer

	// conn is the session this block created and owns. The registry and the
	// discovery watchers both read through it; nothing else in the process
	// holds it, so Close is what releases it.
	conn *zk.Conn
}

// newZkBackend builds the session (probing the ensemble when Ping is set) and
// both halves. The probe is the fail-fast: a misconfigured or unreachable
// ensemble fails startup here, once per block; with Ping off (the default)
// construction skips it.
func newZkBackend(c ZookeeperConfig, name string) (*zkBackend, error) {
	conn, err := connectZookeeper(c)
	if err != nil {
		return nil, err
	}
	obs, err := discovery.NewObserver(obsSystem, name)
	if err != nil {
		conn.Close()
		return nil, err
	}
	reg, err := newZookeeperRegistry(c, conn, obs)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return &zkBackend{
		reg: reg,
		disc: &zkDiscovery{
			conn:     conn,
			basePath: strings.TrimRight(c.BasePath, "/"),
			done:     make(chan struct{}),
			obs:      obs,
			entries:  map[string]*serviceEntry{},
		},
		obs:  obs,
		conn: conn,
	}, nil
}

// Close releases the block's session. It is the bean destructor. The background
// loops are stopped first, so they retire on their own signal instead of
// racing the session close — the discovery watchers in particular cannot infer
// shutdown from a connection error, which is indistinguishable from a
// reconnect they are expected to survive.
func (b *zkBackend) Close() error {
	if b == nil || b.reg == nil {
		return nil
	}
	b.reg.Close()
	b.disc.Close()
	b.conn.Close()
	// Drop this block's gauge callbacks with it: the block owns them, so nothing
	// of it may keep reporting after it is gone.
	_ = b.obs.Close()
	return nil
}

// Register publishes inst into this ensemble (the ephemeral-znode protocol,
// registry.go).
func (b *zkBackend) Register(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Register(ctx, inst)
}

// Deregister removes inst from this ensemble. Idempotent.
func (b *zkBackend) Deregister(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Deregister(ctx, inst)
}

// UpdateWeight re-advertises inst with a new weight.
func (b *zkBackend) UpdateWeight(ctx context.Context, inst discovery.Instance, weight int) error {
	return b.reg.UpdateWeight(ctx, inst, weight)
}

// Resolve serves snapshots of the instances published under this ensemble's
// base path.
func (b *zkBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	return b.disc.Resolve(ctx, name, opts...)
}

// probe reports this ensemble's health for the block's health.Indicator: one
// Exists call, the same check the startup probe runs, repeated on demand.
func (b *zkBackend) probe(ctx context.Context) error {
	if _, _, err := b.reg.conn.Exists("/"); err != nil {
		return errutil.Explain(err, "discovery-zookeeper: probe ensemble")
	}
	return nil
}

// connectZookeeper dials the ensemble for c, applies digest auth when set, and
// when Ping is set probes it (an Exists call blocks until the session connects)
// so an unreachable ensemble fails startup rather than surfacing on the first
// operation.
func connectZookeeper(c ZookeeperConfig) (*zk.Conn, error) {
	if len(c.Servers) == 0 {
		return nil, errutil.Explain(nil, "discovery-zookeeper: servers is required")
	}
	conn, _, err := zk.Connect(c.Servers, c.SessionTimeout)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "connect zookeeper servers=%v failed: %v", c.Servers, err)
		return nil, errutil.Explain(err, "discovery-zookeeper: connect to %v", c.Servers)
	}
	if c.Username != "" || c.Password != "" {
		if err := conn.AddAuth("digest", []byte(c.Username+":"+c.Password)); err != nil {
			conn.Close()
			return nil, errutil.Explain(err, "discovery-zookeeper: add digest auth")
		}
	}
	if c.Ping {
		// Fail-fast probe: an Exists call blocks until the session connects (or
		// the session timeout elapses), so an unreachable ensemble surfaces at
		// boot.
		if _, _, err := conn.Exists("/"); err != nil {
			conn.Close()
			return nil, errutil.Explain(err, "discovery-zookeeper: startup probe failed for %v", c.Servers)
		}
	}
	return conn, nil
}

func init() {
	// One NAMED bean per block under ${spring.discovery.zookeeper.<name>}: the
	// bean name "zookeeper.<name>" is the label a client starter cites to pick
	// this backend for discovery, and the discovery registration core collects the
	// same bean (as a discovery.Registry) into the single publication
	// lifecycle. Blocks across backends never collide (the name carries the
	// backend type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.discovery.zookeeper"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.zookeeper}", func(name string, c ZookeeperConfig) error {
			r.Provide(newZkBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
				gs.IndexArg(1, gs.ValueArg(name)),
			).Name("zookeeper."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registry]()).
				Destroy((*zkBackend).Close).Caller(1)

			// Contribute a health indicator for this ensemble unless the user
			// disabled it (health=false), injecting the backend registered
			// above by name.
			if c.Health {
				r.Provide(func(b *zkBackend) *health.Indicator {
					return &health.Indicator{Name: "discovery-zookeeper:" + name, Probe: b.probe}
				}, gs.TagArg("zookeeper."+name)).Name("discovery-zookeeper:" + name)
			}
			return nil
		})
	})
}
