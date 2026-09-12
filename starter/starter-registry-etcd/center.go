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

package StarterRegistryEtcd

import (
	"context"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

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
		reg:  reg,
		disc: &etcdDiscovery{client: cli, keyPrefix: c.KeyPrefix, bgCtx: context.Background(), entries: map[string]*serviceEntry{}},
	}, nil
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
			return nil
		})
	})
}
