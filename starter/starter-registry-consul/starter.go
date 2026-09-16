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

// Package StarterRegistryConsul adapts Consul as a service registry. Each
// ${spring.registry.consul.<name>} block becomes ONE backend bean named
// "consul.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registrar collected by the starter-registry core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name).
// Blank-import the package and configure one block per Consul agent:
//
//	spring.registry.consul.main.address=127.0.0.1:8500
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-registry-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to Consul.
//
// Importing this package imports the starter-registry registration core
// transitively: the single registryServer bean that publishes into every
// configured center (across backends) exists exactly once per process.
package StarterRegistryConsul

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
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
const obsSystem = "consul"

// starterTag identifies logs emitted by the consul registry starter.
var starterTag = log.RegisterAppTag("registry_consul", "")

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

			// Contribute a health indicator for this agent unless the user
			// disabled it (health.enabled=false), injecting the backend
			// registered above by name.
			if c.HealthEnabled {
				r.Provide(func(b *consulBackend) *health.Indicator {
					return &health.Indicator{Name: "registry-consul:" + name, Probe: b.probe}
				}, gs.TagArg("consul."+name)).Name("registry-consul:" + name)
			}
			return nil
		})
	})
}

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
	client, err := newConsulClient(c)
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
	disc := newConsulDiscovery(client, "")
	log.Debugf(context.Background(), starterTag, "consul backend for address=%s ready", c.Address)
	return &consulBackend{reg: reg, disc: disc}, nil
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

// probe reports this agent's health for the block's health.Indicator: one
// catalog listing, the same check the startup probe runs, repeated on demand.
func (b *consulBackend) probe(ctx context.Context) error {
	if _, _, err := b.reg.client.Catalog().Services((&api.QueryOptions{}).WithContext(ctx)); err != nil {
		return errutil.Explain(err, "registry-consul: probe agent")
	}
	return nil
}

// newConsulClient builds a Consul api client for the block's connection and
// TLS fields. The shared tls.* block maps onto the Consul SDK's own TLS
// fields only when enabled, so an untouched config behaves exactly as before;
// a misconfigured pair (e.g. cert without key) fails in the SDK's transport
// setup and surfaces through the startup probe.
func newConsulClient(c ConsulConfig) (*api.Client, error) {
	cfg := &api.Config{
		Address:    c.Address,
		Scheme:     c.Scheme,
		Datacenter: c.Datacenter,
		Token:      c.Token,
		Namespace:  c.Namespace,
	}
	if c.TLS.Enabled {
		cfg.TLSConfig = api.TLSConfig{
			CAFile:             c.TLS.CAFile,
			CertFile:           c.TLS.CertFile,
			KeyFile:            c.TLS.KeyFile,
			InsecureSkipVerify: c.TLS.InsecureSkipVerify,
		}
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create consul client for address=%s failed: %v", c.Address, err)
		return nil, errutil.Explain(err, "registry-consul: create client for %s", c.Address)
	}
	return client, nil
}
