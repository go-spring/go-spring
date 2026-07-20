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
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

// nacosRegistrar publishes instances to a Nacos naming service. Instances are
// registered as ephemeral, so the Nacos SDK keeps them alive with its own
// background heartbeat and Nacos drops them automatically if the process dies
// without Deregister — no heartbeat goroutine is needed here.
// It implements discovery.Registrar.
type nacosRegistrar struct {
	client  naming_client.INamingClient
	group   string
	cluster string
}

// newNacosRegistrar builds a registrar backed by a Nacos naming client for c.
// It probes the server before returning so an unreachable or misconfigured
// Nacos fails startup rather than surfacing on the first Register.
func newNacosRegistrar(c NacosConfig) (*nacosRegistrar, error) {
	host, portStr, err := net.SplitHostPort(c.Server)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q must be host:port", c.Server)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: server %q has a non-numeric port", c.Server)
	}

	sc := []constant.ServerConfig{*constant.NewServerConfig(host, port)}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(c.Namespace),
		constant.WithTimeoutMs(c.TimeoutMs),
		constant.WithUsername(c.Username),
		constant.WithPassword(c.Password),
		constant.WithNotLoadCacheAtStart(true),
	)
	client, err := clients.NewNamingClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		return nil, errutil.Explain(err, "registry-nacos: create naming client for %s", c.Server)
	}

	// Fail-fast probe: listing services proves the server is reachable and the
	// credentials/namespace are valid, so a bad configuration surfaces at boot.
	if _, err := client.GetAllServicesInfo(vo.GetAllServiceInfoParam{
		NameSpace: c.Namespace,
		GroupName: c.Group,
		PageNo:    1,
		PageSize:  1,
	}); err != nil {
		return nil, errutil.Explain(err, "registry-nacos: startup probe failed for %s", c.Server)
	}

	return &nacosRegistrar{client: client, group: c.Group, cluster: c.Cluster}, nil
}

// Register publishes reg as an ephemeral Nacos instance. The SDK then keeps it
// alive with its own heartbeat until Deregister. Registering the same ip:port
// again refreshes the entry.
func (r *nacosRegistrar) Register(_ context.Context, reg discovery.Registration) error {
	host, port, err := splitAddr(reg.Addr)
	if err != nil {
		return err
	}
	// Nacos treats weight 0 as "receive no traffic"; default to 1 so an
	// unweighted instance is actually reachable.
	weight := float64(reg.Weight)
	if weight <= 0 {
		weight = 1
	}
	ok, err := r.client.RegisterInstance(vo.RegisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: reg.ServiceName,
		GroupName:   r.group,
		ClusterName: r.cluster,
		Weight:      weight,
		Enable:      true,
		Healthy:     true,
		Ephemeral:   true,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-nacos: register %q", reg.ServiceName)
	}
	if !ok {
		return errutil.Explain(nil, "registry-nacos: register %q was rejected by the server", reg.ServiceName)
	}
	return nil
}

// Deregister removes the instance. It is idempotent: deregistering an instance
// that is not registered is a no-op on the Nacos side.
func (r *nacosRegistrar) Deregister(_ context.Context, reg discovery.Registration) error {
	host, port, err := splitAddr(reg.Addr)
	if err != nil {
		return err
	}
	if _, err := r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: reg.ServiceName,
		GroupName:   r.group,
		Cluster:     r.cluster,
		Ephemeral:   true,
	}); err != nil {
		return errutil.Explain(err, "registry-nacos: deregister %q", reg.ServiceName)
	}
	return nil
}

// splitAddr splits a "host:port" advertised address into a host and numeric
// port, returning an explanatory error on either malformation.
func splitAddr(addr string) (string, uint64, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, errutil.Explain(err, "registry-nacos: addr %q must be host:port", addr)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return "", 0, errutil.Explain(err, "registry-nacos: addr %q has a non-numeric port", addr)
	}
	return host, port, nil
}
