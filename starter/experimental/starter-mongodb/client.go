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
	"net"

	"go-spring.org/cloud/governance/label"
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

	// serviceLabel is the service key ("mongodb:<service-name or uri>") that
	// addresses the service's endpoint selection in the governance rules
	// document.
	serviceLabel string
}

// NewClient builds a complete Client — identity and observe — over a connected
// raw client, fixing its config. raw must be ready for use (its command monitor
// already applied) — it is normally the product of newClient's construction.
//
// A mongodb client has no seam for the resilience executor (the mongo driver v2
// offers no per-command hook, and a dial seam would key protection on a
// connection rather than an operation), so this starter is observability-only:
// the command-layer monitor emits the span, metrics and access log, and nothing
// throttles, retries or circuit-breaks. There is no Init step — building a
// Client and initializing it are the same act.
func NewClient(raw *mongo.Client, cfg Config) *Client {
	return &Client{
		Client:       raw,
		obs:          newDBObserver(),
		cfg:          cfg,
		serviceLabel: serviceLabel(cfg),
	}
}

// serviceLabel derives a stable, human-readable service key for a client, so
// limiter and breaker state is scoped per MongoDB instance rather than per
// connection. The same label is what a spring.governance.client.rules[N] rule matches to drive
// this client's endpoint selection (balancer / outlier suspension).
func serviceLabel(c Config) string {
	return label.ServiceLabel("mongodb", c.ServiceName, c.URI)
}

// dialerWrapper adapts a dial function (plain, discovery-backed, or
// resilience-wrapped) to the mongo driver's options.Dialer interface. The
// dialed address is taken as-is so the underlying function (which may itself
// ignore it in favor of a discovery-picked endpoint) decides the target.
type dialerWrapper struct {
	dial dialFunc
}

func (d *dialerWrapper) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return d.dial(ctx, network, address)
}

// Destroy is the gs destroy method: it disconnects the underlying client.
// Discovery runs inside the backend (the loader has no resources) and the
// endpoint-selection binding is one-way, released with the loadbalance
// authority, so there is nothing else to release.
func (o *Client) Destroy() error {
	return o.Disconnect(context.Background())
}
