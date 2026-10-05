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
// wrapper MongoDB clients are injected as, plus its lifecycle, service label,
// the embedded *mongo.Client, and the dialerWrapper adaptor the driver
// holds. It mirrors starter-go-redis's client.go; the per-command observation
// layers live in observe.go and command.go.

package StarterMongoDB

import (
	"context"
	"go-spring.org/cloud/chain"
	"net"

	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Client is the wrapper bean MongoDB clients are injected as. The raw
// *mongo.Client is embedded, not held in an unexported field: both seams are
// installed on that client itself — its options carry the observe command
// monitor and the shared dialer the resilience layer wraps — so there is
// nothing to intercept in the wrapper, and the driver's whole surface is
// promoted as-is. [NewClient] remains the only constructor, so a client can
// never exist without its config.
//
// The resilience seam is the dial layer: the mongo driver v2 exposes no single
// per-operation hook comparable to go-redis's ProcessHook, so the cleanest
// insertion point is the dialer (a breaker trips on connection failures, a
// limiter caps connection churn, a bulkhead bounds concurrent dials).
// Already-open connections run at full speed — this is connection-level
// protection, not per-command. newClient installs a shared dialer instance; the
// constructor ([NewClient]) resolves the executor and newClient then mutates the
// dialer's dial function to the resilience-wrapped one while the client is being
// built, so the swap takes effect without rebuilding the client.
type Client struct {
	// *mongo.Client is the raw client, embedded so its whole surface is
	// promoted. The observe layer (its command monitor) and the resilience
	// layer (its dialer) ride the client's own options, so nothing here needs
	// to re-declare the driver's methods.
	*mongo.Client

	// obs emits the per-command span/metric/access-log triple. Built by
	// [NewClient]: it needs no external input, so it is part of what the client
	// IS, not a later assembly step. The command monitor reads it lazily (see
	// command.go).
	obs *dbObserver

	// cfg is the connection config, retained for the resilience service label.
	cfg Config

	// exec is the resilience executor protecting dials, resolved by [NewClient]
	// and consumed by newClient, which wraps the shared dialer with it. It is
	// never nil: a client with no governance degrades to an observed-only,
	// loudly-unmanaged executor rather than a no-op.
	exec chain.Executor
	// serviceLabel is the resilience service key ("mongodb:<service-name or
	// uri>") exec scopes limiter/breaker state by. The same label addresses the
	// service's endpoint selection in the governance rules document.
	serviceLabel string
	// stop detaches the discovery pool's endpoint-selection binding; nil when
	// discovery is not in effect. Set by newClient, which is where the pool
	// first exists.
	stop func()
}

// NewClient builds a complete Client — identity, observe and governance — over a
// connected raw client, fixing its config. raw must be ready for use (its
// command monitor already applied) — it is normally the product of newClient's
// construction.
//
// params carries the container's facilities (see [cloud.ClientParams]) and is
// applied HERE, so a Client cannot exist half-assembled: this constructor
// derives the service label and resolves the executor
// ([cloud.ClientParams.ExecutorFor]) that scopes the instance's limiter/breaker
// state. A hand-built client passes the zero [cloud.ClientParams]; its executor
// then degrades to
// [resilience.Unmanaged] — observed, with a one-time warning that no protection
// applies — rather than silently running bare.
//
// Because this starter's protection rides the dial layer (the mongo driver v2
// offers no per-command hook), the executor is not consumed here: newClient
// wraps the shared dialer's dial function with [Client.exec] right after this
// returns. There is no Init step — building a Client and initializing it are the
// same act, done in one place, so the container has no lifecycle hook to
// register and no way to hand out a half-built client.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(raw *mongo.Client, cfg Config, params cloud.ClientParams) *Client {
	label := serviceLabel(cfg)
	return &Client{
		Client:       raw,
		obs:          newDBObserver(),
		cfg:          cfg,
		serviceLabel: label,
		exec:         params.ExecutorFor("mongodb", label),
	}
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

// Destroy is the gs destroy method: it detaches the discovery pool's binding,
// closes the resilience executor and disconnects the underlying client.
// Discovery runs inside the backend (the loader has no resources), so nothing
// discovery-related is released besides the binding.
func (o *Client) Destroy() error {
	if o.stop != nil {
		o.stop()
	}
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Disconnect(context.Background())
}
