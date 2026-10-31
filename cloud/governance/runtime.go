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

package governance

import (
	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/observe"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/mesh"
)

// Runtime is the process's data-plane composition root: the one object a client
// or server starter injects to reach any authority it needs.
//
// It holds DATA-PLANE authorities only: the outbound resilience manager, the
// inbound admission manager, the endpoint-selection manager, fault injection in
// both directions, and the in-process discovery directory. The control plane is
// deliberately absent: policy and fault are two peer config roots, each with
// its own center, and they push into these same manager instances without the
// runtime knowing either of them. That keeps the control plane out of the
// request path's object graph and leaves the runtime a pure stack of layers.
//
// Its exported fields are the dependencies, declared rather than passed: the
// container injects them through the autowire tags, and a hand-built runtime (an
// embedder, a test) sets the same fields in a literal. Every one of them is a
// bean registered by the package that owns it.
//
// It is immutable after wiring, so it is safe for concurrent use.
type Runtime struct {
	// Resilience is the OUTBOUND resilience authority the center drives and
	// [Runtime.ExecutorFor] builds on. Required.
	Resilience *resilience.Manager `autowire:""`

	// LoadBalance is the endpoint-selection authority. Required.
	LoadBalance *loadbalance.Manager `autowire:""`

	// ClientFault is the OUTBOUND fault authority, registered by its own
	// starter. Required, like the authorities above.
	ClientFault *clientfault.Manager `autowire:""`

	// ServerFault is the INBOUND fault authority. Required, like ClientFault.
	ServerFault *serverfault.Manager `autowire:""`

	// Directory is the in-process discovery directory, registered by the
	// discovery package over every backend bean — an app with no backend gets an
	// empty directory, not a nil one. Required.
	Directory *discovery.Manager `autowire:""`

	// Admission is the INBOUND admission authority: the manager that creates one
	// [admission.Guard] per server instance and pushes each the server policy.
	// Required, registered by the admission package. It is built over the
	// process's cluster limiter when a rate-limit backend contributes one.
	Admission *admission.Manager `autowire:""`

	// Traffic is the process traffic convention handed to every client and
	// server adapter. Required: the traffic package registers a default when the
	// application contributes none.
	Traffic traffic.Propagator `autowire:""`
}

// ResilienceClient returns the OUTBOUND resilience authority. The Runtime is the
// family's sole spokesperson: a client starter injects the ONE *Runtime bean and
// reads whichever authority it needs from here, instead of wiring each authority
// bean in separately. Every getter returns a non-nil authority.
func (r *Runtime) ResilienceClient() *resilience.Manager {
	return r.Resilience
}

// GuardServer returns the INBOUND admission authority — the manager that creates one
// [admission.Guard] per server instance and pushes each the server policy. It is the
// object a server adapter (or a filter library with no bean to inject into)
// passes to [InboundOver].
func (r *Runtime) GuardServer() *admission.Manager {
	return r.Admission
}

// FaultClient returns the OUTBOUND fault authority; see [Runtime.ResilienceClient].
// It is the object client starters wrap their executors with. A server starter
// has no use for it, and vice versa.
func (r *Runtime) FaultClient() *clientfault.Manager {
	return r.ClientFault
}

// FaultServer returns the INBOUND fault authority; see [Runtime.ResilienceClient].
// It is the object server adapters create their handle from.
func (r *Runtime) FaultServer() *serverfault.Manager {
	return r.ServerFault
}

// Loadbalance returns the endpoint-selection authority; see
// [Runtime.ResilienceClient].
func (r *Runtime) Loadbalance() *loadbalance.Manager {
	return r.LoadBalance
}

// TrafficPropagator returns the process traffic convention. Every adapter that
// builds a call's operation reads the call's traffic class through it (see
// [traffic.Propagator.Kind]) and records it on the operation's metadata, so a
// rule matches match.traffic=... like any other dimension. It is never nil.
func (r *Runtime) TrafficPropagator() traffic.Propagator {
	return r.Traffic
}

// Discovery returns the directory of named discovery backends; see
// [Runtime.ResilienceClient]. It is not a policy authority like the other three —
// the runtime holds it so a client reaches discovery through the same injection
// point it reaches everything else.
func (r *Runtime) Discovery() *discovery.Manager {
	return r.Directory
}

// UsesDiscovery reports whether a client entry configured with serviceName should
// run in-process service discovery. It is the ONE place a client asks that
// question, and the ONE place mesh mode is consulted.
//
// Mesh wins: when the process runs behind a sidecar, the sidecar owns service
// discovery AND load balancing, so the client must dial its configured stable
// address (a service DNS name or a fixed endpoint) and leave selection to the
// sidecar. A configured service-name is then simply ignored — it is a harmless
// leftover of a config shared with non-mesh deployments, not an error. Running
// in-process discovery on top of the sidecar would balance twice and put the
// two layers' outlier decisions in conflict.
//
// The decision is a process-level fact: [mesh.Enabled] is a pure function of
// the process environment, so there is nothing to resolve or cache — this is
// simply the one place it is consulted, and it is consulted at construction
// time, not per request.
func (r *Runtime) UsesDiscovery(serviceName string) bool {
	return serviceName != "" && !mesh.Enabled()
}

// ExecutorFor returns the executor a client of the given system, labelled
// service, runs its calls on. It is the ONE place the observe layer and the
// resilience layer are composed — the runtime names observability, the
// resilience authority names neither it nor the control plane, so the two
// packages stay independent and this composition is all that ties them. It is
// the composition a client's constructor needs in one call, so no driver repeats
// it, on the one object a client already injects.
//
// The layers are stacked fault(observe(resilience-executor)): the observer wraps
// the resilience manager's handle (so a rejected or failed call is still traced,
// measured and logged), and fault injection wraps that. The two planes stay
// separate: the observer records the request, while the resilience executor
// publishes its own breaker and budget state from its own package.
//
// "system" is the client's backend name ("memcached", "redigo", ...), which
// clients also declare in their operations; it is a parameter rather than state
// because one runtime serves every kind of client.
func (r *Runtime) ExecutorFor(system, label string) chain.Executor {
	exec := observe.WrapClientExecutor(r.Resilience.ExecutorFor(label), system, label)
	return r.ClientFault.WrapClientExecutor(exec)
}
