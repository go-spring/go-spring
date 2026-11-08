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
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/dataplane/inbound"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/observe"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/mesh"
	"go-spring.org/cloud/traffic"
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

// The authorities above are the Runtime's whole vocabulary: a starter reads the
// field it needs directly (r.Resilience, r.Admission, ...) — the fields are the
// injection point, and a getter in front of them would be a second owner of the
// same access, not a guarantee. The two composition helpers below are the only
// methods, because they are the only places that know two authorities at once.

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
// service, runs its calls on. It is the ONE place the three outbound layers are
// composed — the runtime names observability and fault, the resilience authority
// names neither of the others, so the packages stay independent and this
// composition is all that ties them. It is the composition a client's
// constructor needs in one call, so no driver repeats it, on the one object a
// client already injects.
//
// Each layer holds its own next and delegates through [chain.Next], so the
// stack is written in the order it runs:
//
//	fault        innermost — next is nil, so it fires just before fn
//	resilience   middle    — its retry loop calls the next layer per attempt
//	observe      outermost — the span covers every attempt, gate and fire
//
// Fault's next is nil because it is the last layer: it wraps the fn it hands
// down, and [chain.Next] calls that fn directly. Placing it BEFORE resilience
// would put the fire outside the retry loop and the breaker, which is exactly
// what a fire must flow through to validate the stack. The two planes stay
// separate: the observer records the request, while the resilience executor
// publishes its own breaker and budget state from its own package.
//
// "system" is the client's backend name ("memcached", "redigo", ...), which
// clients also declare in their operations; it is a parameter rather than state
// because one runtime serves every kind of client.
func (r *Runtime) ExecutorFor(system, label string) chain.Executor {
	var exec chain.Executor = r.ClientFault.WrapClientExecutor(nil)
	exec = r.Resilience.Wrap(label, exec)
	return observe.WrapClientExecutor(exec, system, label)
}

// InboundFor returns the pair of inbound objects one route's middleware drives,
// wired together: the observer records the request plane, while the admission
// publishes its own state. It is the composition root's half of the inbound
// seam — the only place that knows both authorities — so no middleware wires
// the observe object to the admission itself. The handle, and the layer order
// it fixes, live in the data plane:
// [go-spring.org/cloud/governance/dataplane/inbound].
func (r *Runtime) InboundFor(system, label string) (*inbound.Inbound, error) {
	return inbound.Over(system, label, r.Admission, r.ServerFault)
}
