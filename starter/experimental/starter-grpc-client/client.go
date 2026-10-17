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

package StarterGrpcClient

import (
	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/discovery"
	"go-spring.org/cloud/runtime/dataplane/label"
	"go-spring.org/cloud/runtime/dataplane/loadbalance"
	"google.golang.org/grpc"
)

// Client dials discovery-backed gRPC connections for an application. It is a
// bean: inject it where a connection is needed, the same way every other client
// starter hands out its client.
//
// It is what makes this package's endpoint selection per-connection state
// rather than package state. gRPC builds resolvers and balancers itself, from a
// target string and a service config, so the discovery directory and the
// selection policy cannot be injection parameters of those builders — but they
// CAN be fields of the object that DIALS, which is this one. Each Dial therefore
// creates its connection's selection, subscribes it to the service's governance
// label, and hands it to the resolver it registers for that connection alone.
//
// The two authorities are read off the runtime — the family's single
// injection point — so a client reaches discovery and endpoint selection
// exactly like every other client starter does.
type Client struct {
	dir   *discovery.Manager
	lbMgr *loadbalance.Manager
}

// NewClient builds the dialer over the center's authorities. The center bean is
// provided by cloud/runtime/controlplane/governance, which this package imports, so it is always
// present when the module is linked.
func NewClient(center *runtime.Runtime) (*Client, error) {
	return NewClientFrom(center.Discovery(), center.Loadbalance()), nil
}

// NewClientFrom builds a dialer from authorities a caller already holds — the
// hand-built path, for a program that dials gsdiscovery targets without a
// container. The two are exactly what [NewClient] reads off the center, so a
// container-less caller gets the same behavior and can drive governance itself
// by pushing [loadbalance.Settings] into the manager.
func NewClientFrom(dir *discovery.Manager, lbMgr *loadbalance.Manager) *Client {
	return &Client{dir: dir, lbMgr: lbMgr}
}

// GovernLabel is the governance label a connection to service follows. It is the
// label [Client.Dial] registers under, exported so a rule can be written against
// exactly it — `service: grpc:<service>` — instead of a hand-spelled copy that
// may drift.
func GovernLabel(service string) string {
	return label.ServiceLabel("grpc", service)
}

// Dial opens a connection to service.
//
// service is the discovery name, resolved through the connection's own discovery
// backend — the bean named by backend, or "default" when it is empty. The
// governance label is derived from the service ([label.ServiceLabel]), so a
// rule naming "grpc:<service>" scopes this connection's strategy and outlier
// suspension, and is REBUILT per push rather than retuned: the connection's
// [loadbalance.Chain] registers under the label, and the chain's inner
// generation moves with the manager's settings.
//
// opts are passed through to gRPC last, so an application may override anything
// this method sets — including the service config, which is how it would select
// a balancer of its own instead of this package's.
//
// Like [grpc.NewClient] it returns without connecting: gRPC establishes
// connections lazily, on the first RPC (or an explicit Connect).
func (c *Client) Dial(service string, backend string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// The selection chain: built over the READY-snapshot source and registered
	// with the manager — which IS the subscription: the manager hands the chain
	// the selection in force for the label now, and rebuilds its generation on
	// every change. Nothing process-wide is touched, so two connections to
	// different services (or two containers in one process) never share a policy.
	sel := newConnState(GovernLabel(service), c.lbMgr)

	target := Scheme + ":///" + service
	if backend != "" && backend != "default" {
		target = Scheme + "://" + backend + "/" + service
	}

	dopts := []grpc.DialOption{
		// The resolver is registered for THIS connection: it carries the discovery
		// directory and this connection's selection, so neither has to live in
		// package state (see the file comment in balancer.go).
		grpc.WithResolvers(resolverBuilder{dir: c.dir, sel: sel}),
		// The one process-wide registration this package needs (gRPC has no
		// per-connection balancer registry); the builder is stateless, and the
		// selection above is what it will actually apply.
		grpc.WithDefaultServiceConfig(serviceConfig),
	}
	return grpc.NewClient(target, append(dopts, opts...)...)
}

// serviceConfig selects the stateless [balancerName] balancer. The strategy is
// not encoded here — it travels with the connection's selection, which the
// resolver attached to the addresses (see [balancerName]).
const serviceConfig = `{"loadBalancingConfig":[{"` + balancerName + `":{}}]}`
