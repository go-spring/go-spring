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

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

// instance is the instance this process advertises to the Nacos registry. It is
// the local write-side value built from RegistrationConfig in Server.Run; the
// crash-safety contract every registry starter follows lives in starter/DESIGN
// instance is the process advertisement the registrar publishes; it is the
// neutral [discovery.Instance] contract. The crash-safety contract every
// registry starter follows lives in starter/DESIGN §3 (Register must
// self-renew so correctness never depends on Deregister).
type instance = discovery.Instance

// nacosRegistrar publishes instances to a Nacos naming service. Instances are
// registered as ephemeral, so the Nacos SDK keeps them alive with its own
// background heartbeat and Nacos drops them automatically if the process dies
// without Deregister — no heartbeat goroutine is needed here.
type nacosRegistrar struct {
	client  naming_client.INamingClient
	group   string
	cluster string
}

// newNacosRegistrar returns a registrar writing through client (the shared
// center client; the server was already probed when client was built) with
// c's group/cluster. It does NOT close client — the owner (nacosCenter) does.
func newNacosRegistrar(c NacosConfig, client naming_client.INamingClient) (*nacosRegistrar, error) {
	if client == nil {
		return nil, errutil.Explain(nil, "registry-nacos: nil naming client")
	}
	return &nacosRegistrar{client: client, group: c.Group, cluster: c.Cluster}, nil
}

// Register publishes reg as an ephemeral Nacos instance. The SDK then keeps it
// alive with its own heartbeat until Deregister. Registering the same ip:port
// again refreshes the entry.
func (r *nacosRegistrar) Register(_ context.Context, reg instance) error {
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
func (r *nacosRegistrar) Deregister(_ context.Context, reg instance) error {
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

// UpdateWeight re-publishes reg with a new weight via the naming service's
// UpdateInstance. Because the instance stays ephemeral, its heartbeat keeps
// running and discovery subscribers (the Nacos push channel) receive the new
// weight without any re-registration. Mirrors the etcd registrar's
// UpdateWeight so both starters offer the same optional hot-reload API.
func (r *nacosRegistrar) UpdateWeight(_ context.Context, reg instance, weight int) error {
	host, port, err := splitAddr(reg.Addr)
	if err != nil {
		return err
	}
	// Weight 0 is the drain signal and passes through — Nacos natively treats
	// 0 as "receive no traffic". Only a negative (misconfigured) weight is
	// normalized to 1; unlike Register, an unset default never reaches here.
	w := float64(weight)
	if w < 0 {
		w = 1
	}
	ok, err := r.client.UpdateInstance(vo.UpdateInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: reg.ServiceName,
		GroupName:   r.group,
		ClusterName: r.cluster,
		Weight:      w,
		Enable:      true,
		Ephemeral:   true,
		Metadata:    reg.Metadata,
	})
	if err != nil {
		return errutil.Explain(err, "registry-nacos: update weight %q", reg.ServiceName)
	}
	if !ok {
		return errutil.Explain(nil, "registry-nacos: update weight %q was rejected by the server", reg.ServiceName)
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
