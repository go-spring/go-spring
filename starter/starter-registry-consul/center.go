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

package StarterRegistryConsul

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// consulBackend is ONE configured registry center: the
// ${spring.registry.consul.<name>} block made a bean. It owns the single
// Consul api client for that agent (probed at construction) and serves both
// halves of the naming idiom through it: the write side (a
// discovery.Registrar collected by the starter-registry core) and the read
// side (a discovery.Discovery consumers cite by the bean's name
// "consul.<name>"; lazy, so an app that never cites it pays nothing for the
// read half). Both sides resolve the same agent, so read and write can never
// diverge.
//
// The Consul api client holds no resources that need releasing (it is an HTTP
// client wrapper with no Close), so the bean destructor only stops the
// discovery half's background queries.
type consulBackend struct {
	reg  *consulRegistrar
	disc *consulDiscovery
}

// newConsulBackend builds the client and probes the agent. The probe is the
// fail-fast: a misconfigured or unreachable agent fails startup here, once
// per block.
func newConsulBackend(c ConsulConfig) (*consulBackend, error) {
	if c.Address == "" {
		return nil, errutil.Explain(nil, "registry-consul: address is required")
	}
	client, err := newConsulClient(c.Address, c.Scheme, c.Datacenter, c.Token, c.Namespace)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, _, err := client.Catalog().Services((&api.QueryOptions{}).WithContext(ctx)); err != nil {
		return nil, errutil.Explain(err, "registry-consul: startup probe failed for %s", c.Address)
	}
	reg, err := newConsulRegistrar(c, client)
	if err != nil {
		return nil, err
	}
	log.Debugf(context.Background(), starterTag, "consul backend for address=%s ready", c.Address)
	return &consulBackend{reg: reg, disc: newConsulDiscovery(client, "")}, nil
}

// Close stops the discovery half's background blocking queries. It is the
// bean destructor; the Consul api client needs no closing.
func (b *consulBackend) Close() error {
	if b == nil || b.disc == nil {
		return nil
	}
	b.disc.Close()
	return nil
}

// Register publishes inst into this agent (the TTL-check heartbeat protocol,
// registrar.go).
func (b *consulBackend) Register(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Register(ctx, inst)
}

// Deregister removes inst from this agent. Idempotent.
func (b *consulBackend) Deregister(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Deregister(ctx, inst)
}

// UpdateWeight re-advertises inst with a new weight.
func (b *consulBackend) UpdateWeight(ctx context.Context, inst discovery.Instance, weight int) error {
	return b.reg.UpdateWeight(ctx, inst, weight)
}

// Resolve serves snapshots of the healthy instances this agent reports.
func (b *consulBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	return b.disc.Resolve(ctx, name, opts...)
}

// newConsulClient builds a Consul api client for the given connection fields.
func newConsulClient(address, scheme, datacenter, token, namespace string) (*api.Client, error) {
	client, err := api.NewClient(&api.Config{
		Address:    address,
		Scheme:     scheme,
		Datacenter: datacenter,
		Token:      token,
		Namespace:  namespace,
	})
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create consul client for address=%s failed: %v", address, err)
		return nil, errutil.Explain(err, "registry-consul: create client for %s", address)
	}
	return client, nil
}

func init() {
	// One NAMED bean per block under ${spring.registry.consul.<name>}: the
	// bean name "consul.<name>" is the label a client starter cites to pick
	// this backend for discovery, and the starter-registry core collects the
	// same bean (as a discovery.Registrar) into the single publication
	// lifecycle. Blocks across backends never collide (the name carries the
	// backend type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.registry.consul"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.registry.consul}", func(name string, c ConsulConfig) error {
			r.Provide(newConsulBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
			).Name("consul."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registrar]()).
				Destroy((*consulBackend).Close).Caller(1)
			return nil
		})
	})
}
