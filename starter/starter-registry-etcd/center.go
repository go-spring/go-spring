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

	clientv3 "go.etcd.io/etcd/client/v3"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// etcdCenter owns the ONE shared *clientv3.Client for the cluster configured
// under ${spring.registry.etcd}. Both halves of the naming idiom derive from
// it: the registrar (the write side, starter.go) and the discovery backend
// auto-derived for the same cluster (below). Constructing the client once —
// instead of one connection per side — is the "one center config, two beans"
// shape: a dual-role application configures the cluster a single time.
//
// The bean's Destroy closes the client, so neither derived side closes it.
type etcdCenter struct {
	client *clientv3.Client
	config EtcdConfig
}

// newEtcdCenter builds the shared client and probes the cluster. The probe is
// the fail-fast both sides relied on when they built their own clients: a
// misconfigured or unreachable cluster fails startup here, once.
func newEtcdCenter(c EtcdConfig) (*etcdCenter, error) {
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
	return &etcdCenter{client: cli, config: c}, nil
}

// Close releases the shared client. It is the bean destructor.
func (e *etcdCenter) Close() error {
	if e == nil || e.client == nil {
		return nil
	}
	return errutil.Explain(e.client.Close(), "registry-etcd: close center client")
}

func init() {
	// The center bean exists exactly when the cluster is configured. It is
	// nullable-injected (TagArg("?")) by the registrar Server and by discovery
	// backends that inherit the center connection, so it is constructed only
	// when at least one side consumes it; its destructor closes the client once.
	//
	// When discovery-name is non-empty (the default "etcd"), the module also
	// derives a discovery backend bean for the same cluster under that label —
	// the "one config block serves both halves" default. A dual-role app then
	// cites discovery=etcd without any ${spring.discovery.etcd} block; a
	// label colliding with another bean name fails loudly in the container.
	gs.Module(gs.OnProperty("spring.registry.etcd.endpoints"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c EtcdConfig
		if err := conf.Bind(p, &c, "${spring.registry.etcd}"); err != nil {
			return errutil.Explain(err, "registry-etcd: bind center config")
		}
		r.Provide(newEtcdCenter,
			gs.IndexArg(0, gs.ValueArg(c)),
		).Destroy((*etcdCenter).Close).Caller(1)

		if name := c.DiscoveryName; name != "" {
			r.Provide(newCenterDiscoveryBackend,
				gs.IndexArg(0, gs.TagArg("?")),
				gs.IndexArg(1, gs.ValueArg(c.KeyPrefix)),
			).Name(name).Caller(1)
		}
		return nil
	})
}

// newCenterDiscoveryBackend serves snapshots for the center's cluster through
// the shared client. It is the discovery bean auto-derived from
// ${spring.registry.etcd}; key-prefix is the center's (the registrar writes
// under it), so read and write can never diverge.
func newCenterDiscoveryBackend(ec *etcdCenter, keyPrefix string) (discovery.Discovery, error) {
	if ec == nil {
		return nil, errutil.Explain(nil, "registry-etcd: center client unavailable")
	}
	log.Debugf(context.Background(), starterTag, "derived etcd discovery backend from center config, endpoints=%v", ec.config.Endpoints)
	return &etcdDiscovery{
		client:    ec.client,
		keyPrefix: keyPrefix,
		bgCtx:     context.Background(),
		entries:   map[string]*serviceEntry{},
	}, nil
}
