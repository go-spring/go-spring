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

// Package StarterDiscoveryEtcd adapts etcd as a service registry. Each
// ${spring.discovery.etcd.<name>} block becomes ONE backend bean named
// "etcd.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registry collected by the discovery registration core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name). Blank-
// import the package and configure one block per etcd cluster:
//
//	spring.discovery.etcd.main.endpoints=127.0.0.1:2379
//	spring.discovery.service-name=orders
//	spring.discovery.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-discovery-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to etcd.
//
// Each instance is written under a key bound to its own lease and kept alive by
// a background keep-alive. If the process dies without deregistering, the lease
// expires and etcd deletes the key - self-healing without a reaper.
//
// The registration core - the single discoveryServer bean that publishes into
// every configured center (across backends) - lives in cloud/discovery, which
// this package imports anyway, so it exists exactly once per process.
package StarterDiscoveryEtcd

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
)

func init() {
	// One NAMED bean per block under ${spring.discovery.etcd.<name>}: the bean
	// name "etcd.<name>" is the label a client starter cites to pick this
	// backend for discovery, and the discovery registration core collects the same
	// bean (as a discovery.Registry) into the single publication lifecycle.
	// Blocks across backends never collide (the name carries the backend
	// type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.discovery.etcd"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.etcd}", func(name string, c EtcdConfig) error {
			if !c.Enabled {
				return nil
			}
			r.Provide(newEtcdBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
			).Name("etcd."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registry]()).
				Destroy((*etcdBackend).Close).Caller(1)

			// Contribute a health indicator for this cluster unless the user
			// disabled it (health=false), injecting the backend registered
			// above by name.
			if c.Health {
				r.Provide(func(b *etcdBackend) *health.Indicator {
					return &health.Indicator{Name: "discovery-etcd:" + name, Probe: b.probe}
				}, gs.TagArg("etcd."+name)).Name("discovery-etcd:" + name)
			}
			return nil
		})
	})
}

var (
	// starterTag identifies logs emitted by the etcd discovery starter.
	starterTag = log.RegisterAppTag("discovery_etcd", "")
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute and log field.
const obsSystem = "etcd"

// etcdBackend is ONE configured discovery center: the ${spring.discovery.etcd.<name>}
// block made a bean. It owns the single etcd client for that cluster (probed at
// construction, closed by the bean destructor) and serves both halves of the
// naming idiom through it:
//
//   - discovery.Registry — the write side. The discovery registration core collects
//     every backend's registry (across all backends) and drives them through
//     one publication lifecycle.
//   - discovery.Discovery — the read side. Consumers cite this bean by its
//     name ("etcd.<name>") through their discovery key; the bean is lazy, so
//     an app that never cites it pays nothing for the read half.
//
// Read and write share the block's key-prefix, so they can never diverge.
type etcdBackend struct {
	reg  *etcdRegistry
	disc *etcdDiscovery

	// obs is this block's observability layer: one observer per configured
	// block, closed by the bean destructor below.
	obs *discovery.Observer

	// bgCancel ends the discovery half's background watches. It is the signal
	// that separates "the backend is closing" from "a watch failed": the watch
	// loop re-arms on failure, so without a lifetime signal of its own it would
	// keep re-arming against a closed client for as long as the process lived.
	bgCancel context.CancelFunc

	// cli is the concrete client this block owns. The data-plane halves hold
	// only the narrow seams (client.go); the control-plane calls that have no
	// behaviour worth faking — the health probe's Status and the bean
	// destructor's Close — stay here, on the real client.
	cli *clientv3.Client

	// endpoints is the block's dial list; kept for the health probe, which
	// targets the first endpoint.
	endpoints []string
}

// newEtcdBackend builds the client and, when Ping is set, probes the cluster.
// The probe is the fail-fast: a misconfigured or unreachable cluster fails
// startup here, once per block. With Ping off (the default) construction skips
// the probe so a cluster that is not up yet does not block startup.
func newEtcdBackend(c EtcdConfig, name string) (*etcdBackend, error) {
	if len(c.Endpoints) == 0 {
		return nil, errutil.Explain(nil, "discovery-etcd: endpoints is required")
	}
	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "discovery-etcd: build TLS")
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   c.Endpoints,
		Username:    c.Username,
		Password:    c.Password,
		DialTimeout: c.DialTimeout,
		TLS:         tlsCfg,
	})
	if err != nil {
		log.Error(context.Background(), starterTag, err, log.Strings("endpoints", c.Endpoints), log.Msg("create etcd client failed"))
		return nil, errutil.Explain(err, "discovery-etcd: failed to create etcd client")
	}
	if c.Ping {
		ctx, cancel := context.WithTimeout(context.Background(), c.DialTimeout)
		defer cancel()
		if _, err := cli.Status(ctx, c.Endpoints[0]); err != nil {
			_ = cli.Close()
			return nil, errutil.Explain(err, "discovery-etcd: startup probe failed for %s", c.Endpoints[0])
		}
	}
	kv := etcdClient{cli}
	obs, err := discovery.NewObserver(obsSystem, name)
	if err != nil {
		_ = cli.Close()
		return nil, err
	}
	reg, err := newEtcdRegistry(c, kv, obs)
	if err != nil {
		_ = cli.Close()
		return nil, err
	}
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &etcdBackend{
		reg:       reg,
		disc:      &etcdDiscovery{client: kv, keyPrefix: c.KeyPrefix, bgCtx: bgCtx, entries: map[string]*serviceEntry{}, obs: obs},
		obs:       obs,
		bgCancel:  bgCancel,
		cli:       cli,
		endpoints: c.Endpoints,
	}, nil
}

// probe reports this cluster's health for the block's health.Indicator: one
// Status call against the first endpoint, the same check the startup probe
// runs, repeated on demand.
func (b *etcdBackend) probe(ctx context.Context) error {
	if _, err := b.cli.Status(ctx, b.endpoints[0]); err != nil {
		return errutil.Explain(err, "discovery-etcd: probe %s", b.endpoints[0])
	}
	return nil
}

// Close releases the block's client. It is the bean destructor.
func (b *etcdBackend) Close() error {
	if b == nil || b.cli == nil {
		return nil
	}
	// Retire the watch loops before the client they read through goes away, so
	// they exit on their lifetime signal rather than reading a closed client.
	b.bgCancel()
	// Drop this block's gauge callbacks with it: the block is the unit that owns
	// them, so nothing of it may keep reporting after it is gone.
	_ = b.obs.Close()
	// Explain(nil, ...) BUILDS an error rather than passing one through, so it
	// must only be reached with a real cause: wrapping unconditionally here
	// reported a failure on every clean shutdown.
	if err := b.cli.Close(); err != nil {
		return errutil.Explain(err, "discovery-etcd: close backend client")
	}
	return nil
}

// Register publishes inst into this cluster (the lease + keep-alive protocol,
// registry.go).
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
