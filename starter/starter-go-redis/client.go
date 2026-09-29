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

// client.go is the "resource entity" concept of this starter — the Client
// wrapper go-redis clients are injected as, plus its lifecycle (Init/Destroy)
// and service label. It mirrors starter-redigo's pool.go: the entity embeds
// the concrete client and owns the resilience executor + the endpoint-selection
// subscription, while the per-command instrumentation layers live in command.go
// (starter-redigo's conn.go analog).
package StarterGoRedis

import (
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
)

// Client is the wrapper bean go-redis clients are injected as. It
// embeds the concrete redis.UniversalClient (a *redis.Client or *redis.ClusterClient
// depending on mode, so methods promote unchanged).
// newClient returns one; gs then calls Init (InitMethod).
//
// mgr and inj are the governance beans gs injects into the constructor (both
// nil in a standalone call). mgr is normalized in Init: an unarmed manager is
// exactly the "governance off" pass-through, while a nil pointer would panic on
// the method call. inj is nil-safe at its use site.
type Client struct {
	redis.UniversalClient

	cfg     Config                    // for serviceLabel (address fields)
	mgr     *resilience.Manager       // injected governance manager
	inj     *fault.Injector           // injected fault injector
	lbMgr   *loadbalance.Manager      // injected endpoint-selection authority
	lbPool  *loadbalance.Pool         // pool the driver handed back, nil when it built none
	exec    resilience.ClientExecutor // from mgr.ClientExecutorFor; no-op when governance is off
	service string
	detach  func() // releases the selection subscription; nil when no pool was bound
}

// Init is the gs InitMethod. It arms the executor from the injected governance
// manager and attaches the per-command hook so every command flows through it.
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the arming order relative to starter-governance's wiring is
// irrelevant; the fault injector wraps it with inj, which is nil-safe (with no
// injector the fault layer is a transparent pass-through). When governance is
// off — an unarmed manager — the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.service = serviceLabel(o.cfg)
	if o.mgr == nil {
		o.mgr = resilience.NewManager()
	}
	exec := fault.WrapClientExecutor(o.mgr.ClientExecutorFor("redis", o.service), o.service, o.inj)
	o.exec = exec
	o.bindSelection()
	// Layer order (go-redis hooks are FIFO — first added is outermost):
	//
	//   redisotel (trace span + metrics) — added by instrument() in newClient, outermost
	//   observeHook (access log)         — added here, inside the span, outside the breaker
	//   resilienceHook (breaker/retry)   — added here, innermost
	//
	// This is the canonical order the whole client-starter family shares
	// (observe → resilience → inner): the access log wraps the resilient call so
	// one log line covers the whole retry loop, and it rides redisotel's span
	// context for trace_id correlation. observeHook is attached before
	// resilienceHook precisely so it sits outside the breaker.
	applyObservability(o.UniversalClient)
	o.AddHook(&resilienceHook{exec: exec, service: o.service})
	return nil
}

// Destroy is the gs destroy method: closes the resilience executor (if armed),
// releases the endpoint-selection subscription (when one was bound), and closes
// the underlying client.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	if o.detach != nil {
		o.detach()
	}
	return o.UniversalClient.Close()
}

// bindSelection subscribes the driver-supplied pick pool to its label's managed
// selection, keeping the detach for [Client.Destroy]. Doing it here rather than
// inside the Driver is what keeps a company Driver unaware of governance: the
// driver's whole obligation is to return the pool it built.
//
// A nil pool (a topology with no per-endpoint pick, e.g. cluster mode) arms
// nothing. Binding an unarmed manager is safe — the subscription is remembered
// and armed once starter-governance's center goes live — so this is correct at
// construction time, before the container has finished wiring.
func (o *Client) bindSelection() {
	if o.lbPool == nil {
		return
	}
	if o.lbMgr == nil {
		o.lbMgr = loadbalance.NewManager()
	}
	o.detach = o.lbMgr.Bind(o.lbPool, o.service)
}

// serviceLabel derives a stable, human-readable resilience service key for a
// client, so limiter and breaker state is scoped per Redis instance rather than
// per command. It falls back across the mode-specific address fields via the
// shared [resilience.ServiceLabel] helper.
func serviceLabel(c Config) string {
	first := ""
	if len(c.Addrs) > 0 {
		first = c.Addrs[0]
	}
	return resilience.ServiceLabel("redis", c.ServiceName, c.MasterName, c.Addr, first)
}
