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

// nacosCenter owns the ONE shared Nacos naming client for the server
// configured under ${spring.registry.nacos}. Both halves derive from it: the
// registrar (the write side, starter.go) and the discovery backend
// auto-derived for the same server (below) — the same "one center config, one
// connection" convergence as starter-registry-etcd's center.
//
// The bean's Close releases the client, so neither derived side closes it.
type nacosCenter struct {
	client *naming_client.NamingClient
	config NacosConfig
}

// newNacosCenter builds the shared naming client and probes the server. The
// probe is the fail-fast both sides relied on when they built their own
// clients: a misconfigured or unreachable Nacos fails startup here, once.
func newNacosCenter(c NacosConfig) (*nacosCenter, error) {
	if c.Server == "" {
		return nil, errutil.Explain(nil, "registry-nacos: server is required")
	}
	client, err := newNamingClient(c.Server, c.Namespace, c.Username, c.Password, c.TimeoutMs, c.Group)
	if err != nil {
		return nil, err
	}
	return &nacosCenter{client: client, config: c}, nil
}

// Close releases the shared client. It is the bean destructor.
func (e *nacosCenter) Close() error {
	if e == nil || e.client == nil {
		return nil
	}
	e.client.CloseClient()
	return nil
}

func init() {
	// The center bean exists exactly when the server is configured. It is
	// nullable-injected (TagArg("?")) by the registrar Server and by discovery
	// backends that inherit the center connection.
	//
	// When discovery-name is non-empty (the default "nacos"), the module also
	// derives a discovery backend bean for the same server under that label —
	// resolving within the center's namespace/group/cluster, so read and write
	// can never diverge (the divergence a dual-block setup could only WARN
	// about disappears instead). A label colliding with another bean name
	// fails loudly in the container.
	gs.Module(gs.OnProperty("spring.registry.nacos.server"), func(r gs.BeanProvider, p flatten.Storage) error {
		var c NacosConfig
		if err := conf.Bind(p, &c, "${spring.registry.nacos}"); err != nil {
			return errutil.Explain(err, "registry-nacos: bind center config")
		}
		r.Provide(newNacosCenter,
			gs.IndexArg(0, gs.ValueArg(c)),
		).Destroy((*nacosCenter).Close).Caller(1)

		if name := c.DiscoveryName; name != "" {
			r.Provide(newCenterDiscoveryBackend,
				gs.IndexArg(0, gs.TagArg("?")),
			).Name(name).Caller(1)
		}
		return nil
	})
}

// newCenterDiscoveryBackend serves snapshots for the center's server through
// the shared naming client, scoped to the center's namespace/group/cluster.
func newCenterDiscoveryBackend(ec *nacosCenter) (discovery.Discovery, error) {
	if ec == nil {
		return nil, errutil.Explain(nil, "registry-nacos: center client unavailable")
	}
	log.Debugf(context.Background(), starterTag, "derived nacos discovery backend from center config, server=%s group=%s", ec.config.Server, ec.config.Group)
	return &nacosDiscovery{client: ec.client, group: ec.config.Group, cluster: ec.config.Cluster}, nil
}

// newNamingClient builds a Nacos naming client for server and probes it with a
// one-service listing. It is the single construction path behind the center,
// the registrar fallback, and standalone discovery blocks.
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
