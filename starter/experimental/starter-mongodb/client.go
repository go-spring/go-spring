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
// resource label, and the dialerWrapper adaptor the driver holds. It mirrors
// starter-go-redis's client.go; the per-command observation layers live in
// command.go.
package StarterMongoDB

import (
	"context"
	"net"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Client is the wrapper bean MongoDB clients are injected as. It
// embeds the concrete *mongo.Client (so every driver method promotes
// unchanged) and field-injects the resilience policy via gs.Dync so it
// hot-reloads on config change. newClient returns one; gs field-injects
// Resilience + Observability, then calls Init (InitMethod) to build
// the observer and, when resilience is enabled, the executor + wrapped dialer.
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
	// Both resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
	// governance center — so this struct has zero coupling to cloud/governance.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// cfg is the connection config, retained for the resilience resource label.
	cfg Config
	// dialer is the shared dialer handed to the driver; Init swaps
	// its dial field to the resilience-wrapped function.
	dialer *dialerWrapper
	// resolver is the discovery watch behind a service-name client (nil when
	// direct/mesh); Close stops it on shutdown.
	resolver *discovery.Resolver
	// obs is the observe observer built by Init from the injected
	// Observability; the command monitor reads it lazily.
	obs atomic.Pointer[observe.Observer]
	// exec is the resilience executor protecting dials, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("mongodb:<...>") exec scopes
	// limiter/breaker state by.
	resource string
}

// dialerWrapper adapts a dial function (plain, discovery-backed, or
// resilience-wrapped) to the mongo driver's options.Dialer interface. The
// dialed address is taken as-is so the underlying function (which may itself
// ignore it in favor of a Resolver pick) decides the target.
type dialerWrapper struct {
	dial func(ctx context.Context, network, address string) (net.Conn, error)
}

func (d *dialerWrapper) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It builds the observe.Observer (needs Observability)
// and resolves the executor through the neutral [resilience.ExecutorFor] seam
// (backed by starter-govern's governance center when imported), wraps it with the
// process-wide fault injector ([fault.InjectorFor], nil-safe), swaps the
// resilience-wrapped dial function into the shared dialer. When governance is off
// the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.obs.Store(observe.NewDB("mongodb", o.Observability))
	o.resource = resilience.ResourceLabel("mongodb", o.cfg.ServiceName, o.cfg.URI)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, "mongodb", o.Observability)
	o.exec = exec
	// Wrap the current (plain/discovery) dial with the policy and swap it into
	// the shared dialer the driver already holds.
	baseDial := o.dialer.dial
	o.dialer.dial = resilience.NewDialer(baseDial, exec, o.resource)
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if armed),
// stops any discovery watch, and disconnects the underlying client.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	stopLiveResolver(o.resolver)
	return o.Client.Disconnect(context.Background())
}
