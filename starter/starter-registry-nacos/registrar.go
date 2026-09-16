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

	"github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/netutil"
)

// instance is the instance this process advertises to the Nacos registry. It is
// the local write-side value built from RegistrationConfig in Server.Run; the
// crash-safety contract every registry starter follows lives in starter/DESIGN

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

// normalizeWeight clamps a misconfigured negative weight to 1 at write time.
// 0 passes through as the drain signal; an unset weight is expressed by the
// ${spring.registry.weight} default, not by this clamp. Register and
// UpdateWeight share it so both write paths agree on the contract.
func normalizeWeight(w int) int {
	if w < 0 {
		return 1
	}
	return w
}

// Register publishes reg as an ephemeral Nacos instance. The SDK then keeps it
// alive with its own heartbeat until Deregister. Registering the same ip:port
// again refreshes the entry.
func (r *nacosRegistrar) Register(ctx context.Context, reg discovery.Instance) error {
	host, port, err := netutil.SplitHostPort(reg.Addr)
	if err != nil {
		return err
	}
	// Nacos treats weight 0 as "receive no traffic", so an explicit 0 is the
	// drain signal and passes through untouched.
	weight := float64(normalizeWeight(reg.Weight))
	attempt := discovery.RegisterAttempt(ctx, obsSystem, reg.ServiceName, discovery.ReasonInitial)
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
		Metadata:    instanceMetadata(reg),
	})
	// A server-side rejection is a failed registration just as much as a
	// transport error, so both fold into the reported outcome.
	if err == nil && !ok {
		err = errutil.Explain(nil, "registry-nacos: register %q was rejected by the server", reg.ServiceName)
	} else if err != nil {
		err = errutil.Explain(err, "registry-nacos: register %q", reg.ServiceName)
	}
	attempt(err)
	return err
}

// Deregister removes the instance. It is idempotent: deregistering an instance
// that is not registered is a no-op on the Nacos side.
func (r *nacosRegistrar) Deregister(ctx context.Context, reg discovery.Instance) error {
	host, port, err := netutil.SplitHostPort(reg.Addr)
	if err != nil {
		return err
	}
	report := discovery.DeregisterAttempt(ctx, obsSystem, reg.ServiceName)
	_, err = r.client.DeregisterInstance(vo.DeregisterInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: reg.ServiceName,
		GroupName:   r.group,
		Cluster:     r.cluster,
		Ephemeral:   true,
	})
	report(err)
	if err != nil {
		return errutil.Explain(err, "registry-nacos: deregister %q", reg.ServiceName)
	}
	return nil
}

// UpdateWeight re-publishes reg with a new weight via the naming service's
// UpdateInstance. Because the instance stays ephemeral, its heartbeat keeps
// running and discovery subscribers (the Nacos push channel) receive the new
// weight without any re-registration. Mirrors the etcd registrar's
// UpdateWeight so both starters offer the same optional hot-reload API.
func (r *nacosRegistrar) UpdateWeight(ctx context.Context, reg discovery.Instance, weight int) error {
	host, port, err := netutil.SplitHostPort(reg.Addr)
	if err != nil {
		return err
	}
	// Weight 0 is the drain signal and passes through — Nacos natively treats
	// 0 as "receive no traffic".
	w := float64(normalizeWeight(weight))
	report := discovery.WeightChange(ctx, obsSystem, reg.ServiceName)
	ok, err := r.client.UpdateInstance(vo.UpdateInstanceParam{
		Ip:          host,
		Port:        port,
		ServiceName: reg.ServiceName,
		GroupName:   r.group,
		ClusterName: r.cluster,
		Weight:      w,
		Enable:      true,
		Ephemeral:   true,
		Metadata:    instanceMetadata(reg),
	})
	if err == nil && !ok {
		err = errutil.Explain(nil, "registry-nacos: update weight %q was rejected by the server", reg.ServiceName)
	} else if err != nil {
		err = errutil.Explain(err, "registry-nacos: update weight %q", reg.ServiceName)
	}
	report(err)
	return err
}

// instanceMetadata folds the instance's routing dimensions (version, zone,
// scheme) into the metadata map handed to Nacos: the SDK has no dedicated
// fields for them, and the discovery read side restores them from these
// reserved keys.
func instanceMetadata(reg discovery.Instance) map[string]string {
	if reg.Version == "" && reg.Zone == "" && reg.Scheme == "" {
		return reg.Metadata
	}
	md := make(map[string]string, len(reg.Metadata)+3)
	for k, v := range reg.Metadata {
		md[k] = v
	}
	if reg.Version != "" {
		md[discovery.MetaKeyVersion] = reg.Version
	}
	if reg.Zone != "" {
		md[discovery.MetaKeyZone] = reg.Zone
	}
	if reg.Scheme != "" {
		md[discovery.MetaKeyScheme] = reg.Scheme
	}
	return md
}

// splitAddr splits a "host:port" advertised address into a host and numeric
// port, returning an explanatory error on either malformation.
