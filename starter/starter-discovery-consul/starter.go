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

// Package StarterDiscoveryConsul adapts Consul as a service registry. Each
// ${spring.discovery.consul.<name>} block becomes ONE backend bean named
// "consul.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registry collected by the discovery registration core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name).
// Blank-import the package and configure one block per Consul agent:
//
//	spring.discovery.consul.main.address=127.0.0.1:8500
//	spring.discovery.service-name=orders
//	spring.discovery.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-discovery-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to Consul.
//
// The registration core - the single discoveryServer bean that publishes into
// every configured center (across backends) - lives in cloud/discovery, which
// this package imports anyway, so it exists exactly once per process.
package StarterDiscoveryConsul

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
)

func init() {
	// One NAMED bean per block under ${spring.discovery.consul.<name>}: the
	// bean name "consul.<name>" is the label a client starter cites to pick
	// this backend for discovery, and the discovery registration core collects the
	// same bean (as a discovery.Registry) into the single publication
	// lifecycle. Blocks across backends never collide (the name carries the
	// backend type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.discovery.consul"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.consul}", func(name string, c ConsulConfig) error {
			if !c.Enabled {
				return nil
			}
			r.Provide(newConsulBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
				gs.IndexArg(1, gs.ValueArg(name)),
			).Name("consul."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registry]()).
				Destroy((*consulBackend).Close).Caller(1)

			// Contribute a health indicator for this agent unless the user
			// disabled it (health=false), injecting the backend registered
			// above by name.
			if c.Health {
				r.Provide(func(b *consulBackend) *health.Indicator {
					return &health.Indicator{Name: "discovery-consul:" + name, Probe: b.probe}
				}, gs.TagArg("consul."+name)).Name("discovery-consul:" + name)
			}
			return nil
		})
	})
}

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute, so one dashboard can compare discovery centers.
const obsSystem = "consul"

// starterTag identifies logs emitted by the consul discovery starter.
var starterTag = log.RegisterAppTag("discovery_consul", "")

// consulBackend is ONE configured discovery center: the
// ${spring.discovery.consul.<name>} block made a bean. It owns the single
// Consul api client for that agent (probed at construction) and serves both
// halves of the naming idiom through it: the write side (a
// discovery.Registry collected by the discovery registration core) and the read
// side (a discovery.Discovery consumers cite by the bean's name
// "consul.<name>"; lazy, so an app that never cites it pays nothing for the
// read half). Both sides resolve the same agent, so read and write can never
// diverge.
//
// The Consul api client holds no resources that need releasing (it is an HTTP
// client wrapper with no Close), so the bean destructor only stops the
// discovery half's background queries.
type consulBackend struct {
	reg  *consulRegistry
	disc *consulDiscovery

	// obs is this block's observability layer: one observer per configured
	// block, closed by the bean destructor below.
	obs *discovery.Observer
}

// newConsulBackend builds the client and, when Ping is set, probes the agent.
// The probe is the fail-fast: a misconfigured or unreachable agent fails
// startup here, once per block. With Ping off (the default) construction skips
// it.
func newConsulBackend(c ConsulConfig, name string) (*consulBackend, error) {
	if c.Address == "" {
		return nil, errutil.Explain(nil, "discovery-consul: address is required")
	}
	client, err := newConsulClient(c)
	if err != nil {
		return nil, err
	}
	if c.Ping {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, _, err := client.Catalog().Services((&api.QueryOptions{}).WithContext(ctx)); err != nil {
			return nil, errutil.Explain(err, "discovery-consul: startup probe failed for %s", c.Address)
		}
	}
	obs, err := discovery.NewObserver(obsSystem, name)
	if err != nil {
		return nil, err
	}
	reg, err := newConsulRegistry(c, client, obs)
	if err != nil {
		return nil, err
	}
	disc := newConsulDiscovery(client, "", obs)
	log.Debug(context.Background(), starterTag, func() []log.Field {
		return []log.Field{log.String("address", c.Address), log.Msg("consul backend ready")}
	})
	return &consulBackend{reg: reg, disc: disc, obs: obs}, nil
}

// Close stops the discovery half's background blocking queries. It is the
// bean destructor; the Consul api client needs no closing.
func (b *consulBackend) Close() error {
	if b == nil || b.disc == nil {
		return nil
	}
	b.disc.Close()
	// Drop this block's gauge callbacks with it: the block owns them, so nothing
	// of it may keep reporting after it is gone.
	_ = b.obs.Close()
	return nil
}

// Register publishes inst into this agent (the TTL-check heartbeat protocol,
// registry.go).
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
		return errutil.Explain(err, "discovery-consul: probe agent")
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
		log.Error(context.Background(), starterTag, err, log.String("address", c.Address), log.Msg("create consul client failed"))
		return nil, errutil.Explain(err, "discovery-consul: create client for %s", c.Address)
	}
	return client, nil
}
