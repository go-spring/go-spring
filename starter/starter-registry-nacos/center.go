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

package StarterRegistryNacos

import (
	"context"
	"net"
	"strconv"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

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
}

// newNacosBackend builds the naming client (probing the server) and both
// halves. The probe is the fail-fast: a misconfigured or unreachable Nacos
// fails startup here, once per block.
func newNacosBackend(c NacosConfig) (*nacosBackend, error) {
	if c.Server == "" {
		return nil, errutil.Explain(nil, "registry-nacos: server is required")
	}
	client, err := newNamingClient(c.Server, c.Namespace, c.Username, c.Password, c.TimeoutMs, c.Group)
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
		reg:  reg,
		disc: newNacosDiscovery(client, c.Group, c.Cluster),
	}, nil
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

// newNamingClient builds a Nacos naming client for server and probes it with a
// one-service listing. It is the single construction path behind the backend.
func newNamingClient(server, namespace, username, password string, timeoutMs uint64, group string) (*naming_client.NamingClient, error) {
	host, portStr, err := net.SplitHostPort(server)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q must be host:port", server)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q has a non-numeric port", server)
	}

	sc := []constant.ServerConfig{*constant.NewServerConfig(host, port)}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(namespace),
		constant.WithTimeoutMs(timeoutMs),
		constant.WithUsername(username),
		constant.WithPassword(password),
		constant.WithNotLoadCacheAtStart(true),
	)
	ic, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create nacos naming client for server=%s failed: %v", server, err)
		return nil, errutil.Explain(err, "registry-nacos: create naming client for %s", server)
	}
	if _, err := ic.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: namespace, GroupName: group, PageNo: 1, PageSize: 1,
	}); err != nil {
		return nil, errutil.Explain(err, "registry-nacos: startup probe failed for %s", server)
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
			return nil
		})
	})
}
