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

// Package cloud is the framework's shared surface: the types a component's
// extension points are written in terms of. It sits ABOVE the capability
// families (discovery, governance, loadbalance, ...) and imports them, so
// nothing under cloud/ may import it back — the root package is the top of this
// module's dependency graph by construction.
//
// It carries no behaviour of its own beyond the small conveniences those shared
// types need (see [ClientParams.ExecutorFor]).
package cloud

import (
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
)

// ClientParams carries everything the container gives a client driver, so the
// driver can assemble a complete client in one step instead of being handed a
// half-built one and patched afterwards.
//
// It is deliberately named for its FORM — the parameters of construction — not
// for a taxonomy of its contents. A name that classified what is inside would
// go stale the moment something outside that classification joined, and the
// contents are expected to grow. Being "the client's construction parameters"
// stays true whatever it comes to hold.
//
// Its split from [Config] (the driver's other bag) is: Config is what the USER
// wrote in properties; ClientParams is what the CONTAINER provides. One is
// declared, the other is injected.
//
// The zero value is meaningful. A client assembled without a container — a
// hand-built one, an example, a test — passes the zero ClientParams, and its
// executor degrades to [resilience.Unmanaged]: the client is still observed and
// says so once, instead of silently running with no protection at all.
type ClientParams struct {
	// Resilience is the authority that hands out a client's executor: the rate
	// limiter, circuit breaker, retry policy and timeouts, and the resilience
	// layer that emits the client's span, metrics and access log.
	Resilience *resilience.Manager

	// Fault injects faults into outbound calls for fault-tolerance drills.
	Fault *fault.Injector

	// Loadbalance is the endpoint-selection authority a driver binds to when its
	// backend routes by service name. A driver that addresses fixed endpoints
	// ignores it.
	Loadbalance *loadbalance.Manager

	// Discovery resolves a service name to a set of endpoints, for a driver whose
	// backend routes by service name. A driver that addresses fixed endpoints
	// ignores it.
	Discovery discovery.Discovery
}

// ExecutorFor returns the executor a client of the given system, labelled
// service, runs its calls on: the governed one when the container is present —
// the resilience authority's executor for that service, wrapped with fault
// injection — and the observed-only [resilience.Unmanaged] one when it is not.
//
// It is the single place that composition lives, so a client starter's
// constructor is one call rather than a copy of this logic. "system" is the
// client's backend name ("memcached", "redigo", ...), which clients also declare
// in their operations; it is a parameter rather than a field because one
// container assembles many kinds of client.
func (p ClientParams) ExecutorFor(system, label string) resilience.ClientExecutor {
	if p.Resilience == nil {
		return resilience.Unmanaged(system, label)
	}
	return fault.WrapClientExecutor(p.Resilience.ClientExecutorFor(system, label), label, p.Fault)
}

// PolicyFor returns the policy governing the labelled service, or the zero
// policy when no container is present.
//
// A driver needs it for the one policy field that cannot reach a running
// executor: [resilience.ClientPolicy.MaxConns] sizes the connection pool, and a
// pool is built once, when the client is constructed. Everything else the policy
// carries is adopted by the executor on refresh and needs no lookup here.
func (p ClientParams) PolicyFor(label string) resilience.ClientPolicy {
	if p.Resilience == nil {
		return resilience.ClientPolicy{}
	}
	return p.Resilience.ClientPolicyFor(label)
}
