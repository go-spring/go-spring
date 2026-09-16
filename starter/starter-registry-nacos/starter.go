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

// Package StarterRegistryNacos adapts Nacos as a service registry. Each
// ${spring.registry.nacos.<name>} block becomes ONE backend bean named
// "nacos.<name>" serving both sides of the naming idiom: the write side (a
// discovery.Registrar collected by the starter-registry core, which registers
// this instance once the app is ready and deregisters it on shutdown) and the
// read side (a discovery.Discovery consumers cite by the bean's name).
// Blank-import the package and configure one block per Nacos server:
//
//	spring.registry.nacos.main.server=127.0.0.1:8848
//	spring.registry.service-name=orders
//	spring.registry.addr=10.0.0.5:8080
//
// It exists for VM / bare-metal / hybrid deployments where the platform does
// not register instances for you. In pure Kubernetes the platform already
// registers every Pod behind a Service, so you would use
// starter-registry-k8s (the family's discovery-only backend) to *discover*
// peers and not register at all. RPC-framework provider
// registration is out of scope and stays framework-native (starter/DESIGN §3);
// this starter publishes a plain instance (any transport) to Nacos. It is the
// registrar counterpart to starter-config-nacos's config role - the two are
// separate starters with separate config prefixes.
//
// Importing this package imports the starter-registry registration core
// transitively: the single registryServer bean that publishes into every
// configured center (across backends) exists exactly once per process.
package StarterRegistryNacos

import (
	"context"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/netutil"

	// The registration core: provides the single registryServer that collects
	// this backend's registrar beans. Go runs its package init exactly once no
	// matter how many backend starters import it.
	_ "go-spring.org/starter-registry"
)

var (
	// starterTag identifies logs emitted by the nacos registry starter.
	starterTag = log.RegisterAppTag("registry_nacos", "")
)

// obsSystem is this backend's value for the discovery instrumentation's
// "system" attribute, so one dashboard can compare registry centers.
const obsSystem = "nacos"

// nacosBackend is ONE configured registry center: the
// ${spring.registry.nacos.<name>} block made a bean. It owns the single Nacos
// naming client for that server (probed at construction, closed by the bean
// destructor) and serves both halves of the naming idiom through it: the
// write side (a discovery.Registrar collected by the starter-registry core)
// and the read side (a discovery.Discovery consumers cite by the bean's name
// "nacos.<name>"; lazy, so an app that never cites it pays nothing for the
// read half). Both sides resolve within the block's namespace/group/cluster,
// so read and write can never diverge.
type nacosBackend struct {
	reg  *nacosRegistrar
	disc *nacosDiscovery

	// namespace/group scope the health probe's one-service listing the same
	// way the startup probe scopes it.
	namespace string
	group     string
}

// newNacosBackend builds the naming client (probing the server) and both
// halves. The probe is the fail-fast: a misconfigured or unreachable Nacos
// fails startup here, once per block.
func newNacosBackend(c NacosConfig) (*nacosBackend, error) {
	if c.Server == "" {
		return nil, errutil.Explain(nil, "registry-nacos: server is required")
	}
	client, err := newNamingClient(c)
	if err != nil {
		return nil, err
	}
	reg, err := newNacosRegistrar(c, client)
	if err != nil {
		client.CloseClient()
		return nil, err
	}
	log.Debugf(context.Background(), starterTag, "nacos backend for server=%s group=%s ready", c.Server, c.Group)
	return &nacosBackend{
		reg:       reg,
		disc:      newNacosDiscovery(client, c.Group, c.Cluster),
		namespace: c.Namespace,
		group:     c.Group,
	}, nil
}

// probe reports this server's health for the block's health.Indicator: a
// one-service listing within the block's namespace/group, the same check the
// startup probe runs, repeated on demand.
func (b *nacosBackend) probe(ctx context.Context) error {
	if _, err := b.reg.client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: b.namespace, GroupName: b.group, PageNo: 1, PageSize: 1,
	}); err != nil {
		return errutil.Explain(err, "registry-nacos: probe server")
	}
	return nil
}

// Close releases the block's client. It is the bean destructor.
func (b *nacosBackend) Close() error {
	if b == nil || b.disc == nil || b.disc.client == nil {
		return nil
	}
	b.disc.client.CloseClient()
	return nil
}

// Register publishes inst into this server's namespace/group.
func (b *nacosBackend) Register(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Register(ctx, inst)
}

// Deregister removes inst from this server. Idempotent.
func (b *nacosBackend) Deregister(ctx context.Context, inst discovery.Instance) error {
	return b.reg.Deregister(ctx, inst)
}

// UpdateWeight re-advertises inst with a new weight.
func (b *nacosBackend) UpdateWeight(ctx context.Context, inst discovery.Instance, weight int) error {
	return b.reg.UpdateWeight(ctx, inst, weight)
}

// Resolve serves snapshots of the instances in this server's
// namespace/group/cluster.
func (b *nacosBackend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	return b.disc.Resolve(ctx, name, opts...)
}

// newNamingClient builds a Nacos naming client for the block's config and
// probes it with a one-service listing. It is the single construction path
// behind the backend. The shared tls.* block maps onto the SDK's TLS option
// only when enabled, so an untouched config behaves exactly as before.
func newNamingClient(c NacosConfig) (*naming_client.NamingClient, error) {
	host, port, err := netutil.SplitHostPort(c.Server)
	if err != nil {
		return nil, err
	}

	sc := []constant.ServerConfig{*constant.NewServerConfig(host, port)}
	opts := []constant.ClientOption{
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(c.TimeoutMs),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
		constant.WithNotLoadCacheAtStart(true),
	}
	if c.TLS.Enabled {
		opts = append(opts, constant.WithTLS(constant.TLSConfig{
			Appointed:          true,
			Enable:             true,
			TrustAll:           c.TLS.InsecureSkipVerify,
			CaFile:             c.TLS.CAFile,
			CertFile:           c.TLS.CertFile,
			KeyFile:            c.TLS.KeyFile,
			ServerNameOverride: c.TLS.ServerName,
		}))
	}
	cc := constant.NewClientConfig(opts...)
	ic, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create nacos naming client for server=%s failed: %v", c.Server, err)
		return nil, errutil.Explain(err, "registry-nacos: create naming client for %s", c.Server)
	}
	if _, err := ic.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: c.Namespace, GroupName: c.Group, PageNo: 1, PageSize: 1,
	}); err != nil {
		return nil, errutil.Explain(err, "registry-nacos: startup probe failed for %s", c.Server)
	}
	// clients.NewNamingClient always returns the concrete *NamingClient.
	client, _ := ic.(*naming_client.NamingClient)
	if client == nil {
		return nil, errutil.Explain(nil, "registry-nacos: naming client is not the concrete *NamingClient")
	}
	return client, nil
}

func init() {
	// One NAMED bean per block under ${spring.registry.nacos.<name>}: the bean
	// name "nacos.<name>" is the label a client starter cites to pick this
	// backend for discovery, and the starter-registry core collects the same
	// bean (as a discovery.Registrar) into the single publication lifecycle.
	// Blocks across backends never collide (the name carries the backend
	// type); a duplicate name within one backend fails loudly in the
	// container.
	gs.Module(gs.OnProperty("spring.registry.nacos"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.registry.nacos}", func(name string, c NacosConfig) error {
			r.Provide(newNacosBackend,
				gs.IndexArg(0, gs.ValueArg(c)),
			).Name("nacos."+name).
				Export(gs.As[discovery.Discovery](), gs.As[discovery.Registrar]()).
				Destroy((*nacosBackend).Close).Caller(1)

			// Contribute a health indicator for this server unless the user
			// disabled it (health.enabled=false), injecting the backend
			// registered above by name.
			if c.HealthEnabled {
				r.Provide(func(b *nacosBackend) *health.Indicator {
					return &health.Indicator{Name: "registry-nacos:" + name, Probe: b.probe}
				}, gs.TagArg("nacos."+name)).Name("registry-nacos:" + name)
			}
			return nil
		})
	})
}
