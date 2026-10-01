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
// wrapper MongoDB clients are injected as, plus its lifecycle (Init/Destroy),
// service label, and the dialerWrapper adaptor the driver holds. It mirrors
// starter-go-redis's client.go; the per-command observation layers live in
// command.go.
package StarterMongoDB

import (
	"context"
	"net"
	"sync/atomic"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go.mongodb.org/mongo-driver/v2/mongo"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client is the wrapper bean MongoDB clients are injected as. It
// embeds the concrete *mongo.Client (so every driver method promotes
// unchanged) and arms its resilience executor from the injected governance
// manager, so the policy comes from the governance document and hot-reloads
// inside the executor. newClient returns one; gs calls
// Init (InitMethod) to build the observer and swap the executor into the dialer.
//
// The resilience seam is the dial layer: the mongo driver v2 exposes no single
// per-operation hook comparable to go-redis's ProcessHook, so the cleanest
// insertion point is the dialer (a breaker trips on connection failures, a
// limiter caps connection churn, a bulkhead bounds concurrent dials).
// Already-open connections run at full speed — this is connection-level
// protection, not per-command. newClient installs a shared dialer instance and
// Init mutates its dial function to the resilience-wrapped one, so
// the swap takes effect without rebuilding the client.
type Client struct {
	*mongo.Client

	// mgr and inj are the governance beans gs injects into the constructor (both
	// nil in a standalone call). mgr is normalized in Init: an unarmed manager is
	// exactly the "governance off" pass-through, while a nil pointer would panic
	// on the method call. inj is nil-safe at its use site.
	mgr *resilience.Manager
	inj *fault.Injector

	// cfg is the connection config, retained for the resilience service label.
	cfg Config
	// dialer is the shared dialer handed to the driver; Init swaps
	// its dial field to the resilience-wrapped function.
	dialer *dialerWrapper
	// obs is the instrumentation built by Init; the command monitor reads it
	// lazily.
	obs atomic.Pointer[dbObserver]
	// exec is the resilience executor protecting dials, resolved from the injected
	// manager; no-op when governance is off.
	exec resilience.ClientExecutor
	// service is the resilience service key ("mongodb:<...>") exec scopes
	// limiter/breaker state by. The same label addresses the service's endpoint
	// selection in the governance rules document.
	service string
	// stop detaches the discovery pool's endpoint-selection binding; nil when
	// discovery is not in effect. Bound by newClient, which is where the pool
	// first exists.
	stop func()
}

// serviceLabel derives a stable, human-readable service key for a client, so
// limiter and breaker state is scoped per MongoDB instance rather than per
// connection. The same label is what a spring.governance.client.rules[N] rule matches to drive
// this client's endpoint selection (balancer / outlier suspension).
func serviceLabel(c Config) string {
	return resilience.ServiceLabel("mongodb", c.ServiceName, c.URI)
}

// dialerWrapper adapts a dial function (plain, discovery-backed, or
// resilience-wrapped) to the mongo driver's options.Dialer interface. The
// dialed address is taken as-is so the underlying function (which may itself
// ignore it in favor of a discovery-picked endpoint) decides the target.
type dialerWrapper struct {
	dial func(ctx context.Context, network, address string) (net.Conn, error)
}

func (d *dialerWrapper) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

// Init is the gs InitMethod: it builds the instrumentation (see observe.go) and
// arms the executor from the injected governance manager, then swaps the
// resilience-wrapped dial function into the shared dialer. The manager's
// ClientExecutorFor resolves its backing executor lazily, on each Execute, so the
// arming order relative to starter-governance's wiring is irrelevant; the fault
// injector wraps it with inj, which is nil-safe (with no injector the fault layer
// is a transparent pass-through). When governance is off — an unarmed manager —
// the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.obs.Store(newDBObserver())
	o.service = serviceLabel(o.cfg)
	exec := fault.WrapClientExecutor(o.mgr.ClientExecutorFor("mongodb", o.service), o.service, o.inj)
	o.exec = exec
	// Wrap the current (plain/discovery) dial with the policy and swap it into
	// the shared dialer the driver already holds.
	baseDial := o.dialer.dial
	o.dialer.dial = resilience.NewDialer(baseDial, exec)
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if armed)
// and disconnects the underlying client. Discovery runs inside the backend (the
// loader has no resources), so nothing discovery-related is released here.
func (o *Client) Destroy() error {
	if o.stop != nil {
		o.stop()
	}
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Client.Disconnect(context.Background())
}
