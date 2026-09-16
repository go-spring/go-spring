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

// Package StarterRegistryEtcd adapts etcd as a service registry. Each
// ${spring.registry.etcd.<name>} block becomes ONE backend bean named
// "etcd.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registrar collected by the starter-registry core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name). Blank-
// import the package and configure one block per etcd cluster:
//
//	spring.registry.etcd.main.endpoints=127.0.0.1:2379
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-registry-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to etcd.
//
// Each instance is written under a key bound to its own lease and kept alive by
// a background keep-alive. If the process dies without deregistering, the lease
// expires and etcd deletes the key - self-healing without a reaper.
//
// Importing this package imports the starter-registry registration core
// transitively: the single registryServer bean that publishes into every
// configured center (across backends) exists exactly once per process.
package StarterRegistryEtcd

import (
	"context"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"

	// The registration core: provides the single registryServer that collects
	// this backend's registrar beans. Go runs its package init exactly once no
	// matter how many backend starters import it.
	_ "go-spring.org/starter-registry"
)

var (
	// starterTag identifies logs emitted by the etcd registry starter.
	starterTag = log.RegisterAppTag("registry_etcd", "")
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute and log field.
const obsSystem = "etcd"

// etcdBackend is ONE configured registry center: the ${spring.registry.etcd.<name>}
// block made a bean. It owns the single etcd client for that cluster (probed at
// construction, closed by the bean destructor) and serves both halves of the
// naming idiom through it:
//
//   - discovery.Registrar — the write side. The starter-registry core collects
//     every backend's registrar (across all backends) and drives them through
//     one publication lifecycle.
//   - discovery.Discovery — the read side. Consumers cite this bean by its
//     name ("etcd.<name>") through their discovery key; the bean is lazy, so
//     an app that never cites it pays nothing for the read half.
//
// Read and write share the block's key-prefix, so they can never diverge.
type etcdBackend struct {
	reg  *etcdRegistrar
	disc *etcdDiscovery

	// endpoints is the block's dial list; kept for the health probe, which
	// targets the first endpoint.
	endpoints []string
}

// newEtcdBackend builds the client and probes the cluster. The probe is the
// fail-fast: a misconfigured or unreachable cluster fails startup here, once
// per block.
func newEtcdBackend(c EtcdConfig) (*etcdBackend, error) {
	if len(c.Endpoints) == 0 {
		return nil, errutil.Explain(nil, "registry-etcd: endpoints is required")
	}
	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "registry-etcd: build TLS")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
		TLS:         tlsCfg,
	})
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create etcd client for endpoints=%v failed: %v", c.Endpoints, err)
		return nil, errutil.Explain(err, "registry-etcd: failed to create etcd client")
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
	defer cancel()
	if _, err := cli.Status(ctx, c.Endpoints[0]); err != nil {
		_ = cli.Close()
		return nil, errutil.Explain(err, "registry-etcd: startup probe failed for %s", c.Endpoints[0])
	}
	reg, err := newEtcdRegistrar(c, cli)
	if err != nil {
		_ = cli.Close()
		return nil, err
	}
	return &etcdBackend{
		reg:       reg,
		disc:      &etcdDiscovery{client: cli, keyPrefix: c.KeyPrefix, bgCtx: context.Background(), entries: map[string]*serviceEntry{}},
		endpoints: c.Endpoints,
	}, nil
}

// probe reports this cluster's health for the block's health.Indicator: one
// Status call against the first endpoint, the same check the startup probe
// runs, repeated on demand.
func (b *etcdBackend) probe(ctx context.Context) error {
	if _, err := b.reg.client.Status(ctx, b.endpoints[0]); err != nil {
		return errutil.Explain(err, "registry-etcd: probe %s", b.endpoints[0])
	}
	return nil
}

// Close releases the block's client. It is the bean destructor.
func (b *etcdBackend) Close() error {
	if b == nil || b.reg == nil {
		return nil
	}
	// Explain(nil, ...) BUILDS an error rather than passing one through, so it
	// must only be reached with a real cause: wrapping unconditionally here
	// reported a failure on every clean shutdown.
	if err := b.reg.client.Close(); err != nil {
		return errutil.Explain(err, "registry-etcd: close backend client")
	}
	return nil
}

// Register publishes inst into this cluster (the lease + keep-alive protocol,
// registrar.go).
func (b *etcdBackend) Register(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Register(ctx, inst)
}

// Deregister removes inst from this cluster. Idempotent.
func (b *etcdBackend) Deregister(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Deregister(ctx, inst)
}

// UpdateWeight re-advertises inst with a new weight on its existing lease.
func (b *etcdBackend) UpdateWeight(ctx context.Context, inst discovery.Instance, weight int) error {
	return b.reg.UpdateWeight(ctx, inst, weight)
}

// Resolve serves snapshots of the instances published under this cluster's
// key prefix.
func (b *etcdBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	return b.disc.Resolve(ctx, name, opts...)
}

func init() {
	// One NAMED bean per block under ${spring.registry.etcd.<name>}: the bean
	// name "etcd.<name>" is the label a client starter cites to pick this
	// backend for discovery, and the starter-registry core collects the same
	// bean (as a discovery.Registrar) into the single publication lifecycle.
	// Blocks across backends never collide (the name carries the backend
	// type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.registry.etcd"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.registry.etcd}", func(name string, c EtcdConfig) error {
			r.Provide(newEtcdBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
			).Name("etcd."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registrar]()).
				Destroy((*etcdBackend).Close).Caller(1)

			// Contribute a health indicator for this cluster unless the user
			// disabled it (health.enabled=false), injecting the backend
			// registered above by name.
			if c.HealthEnabled {
				r.Provide(func(b *etcdBackend) *health.Indicator {
					return &health.Indicator{Name: "registry-etcd:" + name, Probe: b.probe}
				}, gs.TagArg("etcd."+name)).Name("registry-etcd:" + name)
			}
			return nil
		})
	})
}
