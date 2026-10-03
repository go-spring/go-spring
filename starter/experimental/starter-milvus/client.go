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

// client.go is the resource entity + lifecycle of this starter: the Client the
// Milvus connection is injected as. The Milvus SDK is gRPC-based and accepts
// dial options, so the per-RPC resilience guard is installed as a client
// interceptor chain (see guard.go) — every RPC the wrapper's raw client issues
// is protected without opt-in at the call site, matching the transparent
// per-request stance of the other NoSQL starters.
package StarterMilvus

import (
	"context"
	"go-spring.org/cloud/chain"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// Client is the bean Milvus connections are injected as. The raw SDK client is
// embedded, not held in an unexported field: the guard rides gRPC client
// interceptors installed on that client's own dial options, so there is nothing
// to intercept in the wrapper and the SDK's whole interface is promoted as-is.
// [NewClient] remains the only constructor, so a client can never exist without
// its config.
type Client struct {
	// client.Client is the raw SDK client, embedded so its whole interface is
	// promoted. The guard rides the gRPC interceptors on the client's own dial
	// options (see guard.go), so nothing here needs to re-declare the SDK's
	// methods.
	client.Client

	// cfg is the connection config, retained for the resilience service label.
	cfg Config

	// exec is the resilience executor the guard interceptors route through,
	// set by [NewClient] from the governance bundle; it is also the single
	// emitter of each RPC's span, metrics and access log.
	exec chain.Executor
	// serviceLabel is the resilience service key ("milvus:<addr>") exec scopes
	// limiter/breaker state by.
	serviceLabel string
}

// NewClient builds a complete Client — connection, identity, guard and
// governance — from cfg and the container's facilities. It dials the Milvus
// connection itself: the guard is a gRPC interceptor chain carried on the dial
// options (see guard.go), and dial options are fixed the moment the SDK client
// is created, so the ordering "install the guard, then dial" is this
// constructor's to own. That is also why the guard slot stays private — an
// exported seam taking an unexported *guardSlot could not be called from
// outside the package, so it would be an exported name in appearance only.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client passes the zero [cloud.ClientParams]; its executor then
// degrades to resilience.Unmanaged — observed, with a one-time warning that no
// protection applies — rather than silently running bare.
//
// The executor is resolved through [cloud.ClientParams.ExecutorFor] and handed
// to the slot before any RPC can flow, so every subsequent call — the caller's
// startup probe included — already runs under the guard. The manager resolves
// its backing executor lazily, on each Execute, so the call order relative to
// the center's wiring is irrelevant.
func NewClient(ctx context.Context, cfg Config, params cloud.ClientParams) (*Client, error) {
	slot := &guardSlot{}
	raw, err := client.NewClient(ctx, client.Config{
		Address:     cfg.Addr,
		Username:    cfg.Username,
		Password:    cfg.Password,
		DBName:      cfg.Database,
		DialOptions: guardDialOptions(slot),
	})
	if err != nil {
		return nil, errutil.Explain(err, "milvus: create client")
	}
	o := &Client{Client: raw, cfg: cfg}
	o.serviceLabel = resilience.ServiceLabel("milvus", cfg.Addr)
	o.exec = params.ExecutorFor("milvus", o.serviceLabel)
	slot.apply(o.exec)
	return o, nil
}

// Destroy releases the resilience executor (if governance was applied) and
// closes the connection. It is the gs destroy method.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Close()
}
