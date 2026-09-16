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

// Package StarterRegistryZookeeper adapts ZooKeeper as a service registry.
// Each ${spring.registry.zookeeper.<name>} block becomes ONE backend bean
// named "zookeeper.<name>" serving both sides of the naming idiom: the write
// side (a discovery.Registrar collected by the starter-registry core, which
// registers this instance once the app is ready and deregisters it on
// shutdown) and the read side (a discovery.Discovery consumers cite by the
// bean's name). Blank-import the package and configure one block per
// ensemble:
//
//	spring.registry.zookeeper.main.servers=127.0.0.1:2181
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-registry-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to ZooKeeper.
//
// Each instance is written as an ephemeral znode. An ephemeral node lives only
// as long as the client session, so if the process dies without deregistering,
// ZooKeeper removes the node once the session expires - self-healing without a
// reaper.
//
// Importing this package imports the starter-registry registration core
// transitively: the single registryServer bean that publishes into every
// configured center (across backends) exists exactly once per process.
package StarterRegistryZookeeper

import (
	"context"
	"errors"
	"strings"

	"github.com/go-zookeeper/zk"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"

	// The registration core: provides the single registryServer that collects
	// this backend's registrar beans. Go runs its package init exactly once no
	// matter how many backend starters import it.
	_ "go-spring.org/starter-registry"
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute, so one dashboard can compare registry centers.
const obsSystem = "zookeeper"

var (
	// starterTag identifies logs emitted by the zookeeper registry starter.
	starterTag = log.RegisterAppTag("registry_zookeeper", "")
)

// zkBackend is ONE configured registry center: the
// ${spring.registry.zookeeper.<name>} block made a bean. It owns the single
// ZooKeeper session for that ensemble (probed at construction, closed by the
// bean destructor) and serves both halves of the naming idiom through it: the
// write side (a discovery.Registrar collected by the starter-registry core)
// and the read side (a discovery.Discovery consumers cite by the bean's name
// "zookeeper.<name>"; lazy, so an app that never cites it pays nothing for
// the read half). Both sides share the block's base-path, so read and write
// can never diverge.
type zkBackend struct {
	reg  *zkRegistrar
	disc *zkDiscovery
}

// newZkBackend builds the session (probing the ensemble) and both halves. The
// probe is the fail-fast: a misconfigured or unreachable ensemble fails
// startup here, once per block.
func newZkBackend(c ZookeeperConfig) (*zkBackend, error) {
	conn, err := connectZookeeper(c)
	if err != nil {
		return nil, err
	}
	reg, err := newZookeeperRegistrar(c, conn)
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
			entries:  map[string]*serviceEntry{},
		},
	}, nil
}

// Close releases the block's session (stopping the registrar's session
// monitor and the discovery watchers with it). It is the bean destructor.
func (b *zkBackend) Close() error {
	if b == nil || b.reg == nil {
		return nil
	}
	b.reg.Close()
	return nil
}

// Register publishes inst into this ensemble (the ephemeral-znode protocol,
// registrar.go).
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
		return errutil.Explain(err, "registry-zookeeper: probe ensemble")
	}
	return nil
}

// connectZookeeper dials the ensemble for c, applies digest auth when set, and
// probes it (an Exists call blocks until the session connects) so an
// unreachable ensemble fails startup rather than surfacing on the first
// operation.
func connectZookeeper(c ZookeeperConfig) (*zk.Conn, error) {
	if len(c.Servers) == 0 {
		return nil, errutil.Explain(nil, "registry-zookeeper: servers is required")
	}
	conn, _, err := zk.Connect(c.Servers, c.SessionTimeout)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "connect zookeeper servers=%v failed: %v", c.Servers, err)
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

// errConnectionClosed reports whether err means the zk connection is closed
// for good (no retry can succeed).
func errConnectionClosed(err error) bool {
	return errors.Is(err, zk.ErrConnectionClosed) || errors.Is(err, zk.ErrClosing)
}

func init() {
	// One NAMED bean per block under ${spring.registry.zookeeper.<name>}: the
	// bean name "zookeeper.<name>" is the label a client starter cites to pick
	// this backend for discovery, and the starter-registry core collects the
	// same bean (as a discovery.Registrar) into the single publication
	// lifecycle. Blocks across backends never collide (the name carries the
	// backend type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.registry.zookeeper"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.registry.zookeeper}", func(name string, c ZookeeperConfig) error {
			r.Provide(newZkBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
			).Name("zookeeper."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registrar]()).
				Destroy((*zkBackend).Close).Caller(1)

			// Contribute a health indicator for this ensemble unless the user
			// disabled it (health.enabled=false), injecting the backend
			// registered above by name.
			if c.HealthEnabled {
				r.Provide(func(b *zkBackend) *health.Indicator {
					return &health.Indicator{Name: "registry-zookeeper:" + name, Probe: b.probe}
				}, gs.TagArg("zookeeper."+name)).Name("registry-zookeeper:" + name)
			}
			return nil
		})
	})
}
