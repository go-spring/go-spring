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

package runtime

import (
	"go-spring.org/cloud/runtime/controlplane/governance"
	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/runtime/dataplane/inbound/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/runtime/dataplane/outbound/fault"
	"go-spring.org/cloud/runtime/dataplane/outbound/observe"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
)

// Option customizes the authorities a [Runtime] composes.
type Option func(*options)

type options struct {
	limiter ratelimit.ClusterLimiter
}

// WithClusterLimiter makes the inbound admission charge a shared rate-limit store
// instead of its per-instance budget. It is the inbound counterpart of the
// limiter the resilience Driver receives.
func WithClusterLimiter(l ratelimit.ClusterLimiter) Option {
	return func(o *options) { o.limiter = l }
}

// Runtime is the process's governance composition root: the one object a client
// or server starter injects to reach any authority it needs.
//
// It owns the control plane ([governance.Center]) and every data-plane
// authority: the outbound resilience manager, the inbound admission manager, the
// endpoint-selection manager, fault injection in both directions, and the
// in-process discovery directory. The control plane pushes documents to the same
// instances this object hands out, which is what routes a config push to the
// clients and servers that follow it.
//
// It is immutable after construction, so it is safe for concurrent use.
type Runtime struct {
	control *governance.Center

	resClient   *resilience.Manager
	guardServer *admission.Manager
	lb          *loadbalance.Manager
	injClient   *clientfault.ClientManager
	injServer   *serverfault.ServerManager
	disc        *discovery.Manager
}

// New builds a Runtime over the module authorities and arms it in the same
// breath: the source's snapshot is dispatched into the authorities by
// [governance.NewCenter], and a document that cannot serve fails the
// constructor, which is where the wiring can fail startup.
//
// src is required and bound for life; a hand-built runtime passes a
// [governance.PushSource] wrapping the fixed document it wants. The caller owns
// every authority it passes (the container's beans, or a test's fixtures); the
// same instances are what [Runtime] hands back out.
func New(resClient *resilience.Manager,
	lb *loadbalance.Manager, injClient *clientfault.ClientManager, injServer *serverfault.ServerManager,
	disc *discovery.Manager, src governance.Source, opts ...Option) (*Runtime, error) {
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	var guardOpts []admission.Option
	if o.limiter != nil {
		guardOpts = append(guardOpts, admission.WithClusterLimiter(o.limiter))
	}
	gm := admission.NewManager(guardOpts...)
	control, err := governance.NewCenter(resClient, gm, lb, injClient, injServer, src)
	if err != nil {
		_ = gm.Close()
		return nil, err
	}
	return &Runtime{
		control:     control,
		resClient:   resClient,
		guardServer: gm,
		lb:          lb,
		injClient:   injClient,
		injServer:   injServer,
		disc:        disc,
	}, nil
}

// NewProcess builds a Runtime for a process that has no governance document —
// an embedder or a test that wants the resilience and fault authorities behind
// the one injection point a client already takes, without a rule list to serve.
// The load-balance authority is a fresh empty manager and the discovery
// directory is absent; callers that route by service name must join a [Runtime]
// built over a real directory instead.
//
// It is [New] over an empty [governance.PushSource]: the authorities are armed
// against an empty document, so nothing is governed and every label falls
// through to its default — exactly the behaviour of a process configured with no
// rules.
func NewProcess(resClient *resilience.Manager,
	injClient *clientfault.ClientManager, injServer *serverfault.ServerManager, opts ...Option) (*Runtime, error) {
	return New(resClient, loadbalance.NewManager(), injClient, injServer, nil,
		governance.NewPushSource(governance.Config{}), opts...)
}

// Center returns the control-plane authority this runtime drives. It is for
// control-plane operations only ([governance.Center.Validate]); request-path
// code reaches the data-plane authorities through the methods below.
func (r *Runtime) Center() *governance.Center {
	if r == nil {
		return nil
	}
	return r.control
}

// ResilienceClient returns the OUTBOUND resilience authority. The Runtime is the
// family's sole spokesperson: a client starter injects the ONE *Runtime bean and
// reads whichever authority it needs from here, instead of wiring each authority
// bean in separately.
//
// A nil *Runtime reads as nil authorities, exactly as a zero construction bundle
// did before the runtime was the one injection point: a caller that never routes
// through an authority (a hand-built client, a test) may pass nil and only the
// authority it actually reaches for must be present. The composition helpers
// ([Runtime.ExecutorFor] / [Runtime.PolicyFor]) still require a resilience
// manager and panic without one, since a client is never assembled unprotected.
func (r *Runtime) ResilienceClient() *resilience.Manager {
	if r == nil {
		return nil
	}
	return r.resClient
}

// GuardServer returns the INBOUND admission authority — the manager that creates one
// [admission.Guard] per server instance and pushes each the server policy. It is the
// object a server adapter (or a filter library with no bean to inject into)
// passes to [InboundOver].
func (r *Runtime) GuardServer() *admission.Manager {
	if r == nil {
		return nil
	}
	return r.guardServer
}

// FaultClient returns the OUTBOUND fault authority; see [Runtime.ResilienceClient].
// It is the object client starters wrap their executors with. A server starter
// has no use for it, and vice versa.
func (r *Runtime) FaultClient() *clientfault.ClientManager {
	if r == nil {
		return nil
	}
	return r.injClient
}

// FaultServer returns the INBOUND fault authority; see [Runtime.ResilienceClient].
// It is the object server adapters create their shell from.
func (r *Runtime) FaultServer() *serverfault.ServerManager {
	if r == nil {
		return nil
	}
	return r.injServer
}

// Loadbalance returns the endpoint-selection authority; see
// [Runtime.ResilienceClient].
func (r *Runtime) Loadbalance() *loadbalance.Manager {
	if r == nil {
		return nil
	}
	return r.lb
}

// Discovery returns the directory of named discovery backends; see
// [Runtime.ResilienceClient]. It is not a policy authority like the other three —
// the runtime holds it so a client reaches discovery through the same injection
// point it reaches everything else.
func (r *Runtime) Discovery() *discovery.Manager {
	if r == nil {
		return nil
	}
	return r.disc
}

// ExecutorFor returns the executor a client of the given system, labelled
// service, runs its calls on. It is the ONE place the observe layer and the
// governance layer are composed — the runtime names observability, the
// resilience authority names neither it nor the control plane, so the two
// packages stay independent and this composition is all that ties them. It is
// the composition a client's constructor needs in one call, so no driver repeats
// it, on the one object a client already injects.
//
// The layers are stacked fault(observe(resilience-executor)): the observer wraps
// the resilience manager's handle (so a rejected or failed call is still traced,
// measured and logged), and fault injection wraps that. The observe wrapper
// reaches the breaker through the handle's [breaker.EventListenerSetter],
// so breaker transitions still emit.
//
// "system" is the client's backend name ("memcached", "redigo", ...), which
// clients also declare in their operations; it is a parameter rather than state
// because one runtime serves every kind of client.
func (r *Runtime) ExecutorFor(system, label string) chain.Executor {
	exec := observe.WrapClientExecutor(r.resClient.ExecutorFor(system, label), system, label)
	if r.injClient == nil {
		// A nil runtime is the standalone case a caller normalized away: there is
		// no governance, so there is no fault layer either — but observation is
		// unconditional, so the call is still traced and measured.
		return exec
	}
	return r.injClient.WrapClientExecutor(exec)
}

// PolicyFor returns the outbound policy governing the labelled service.
//
// A driver needs it for a field the running executor cannot deliver: one baked
// into the client at construction rather than adopted per call, because a pool
// or a transport is built once. Everything else the policy carries is adopted by
// the executor on refresh and needs no lookup here.
//
// No field is construction-bound today, so nothing calls this yet: it is the
// seam such a knob plugs into when one exists.
func (r *Runtime) PolicyFor(label string) resilience.Policy {
	return r.resClient.PolicyFor(label)
}

// Close releases the authorities this runtime owns in the order that keeps the
// request path silent first: the inbound admission is closed (its breakers and
// admission state), then the control plane closes its bound source. The other
// authorities are beans with their own Destroy hooks, so they are not closed
// here.
func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.guardServer != nil {
		_ = r.guardServer.Close()
	}
	if r.control != nil {
		return r.control.Close()
	}
	return nil
}
