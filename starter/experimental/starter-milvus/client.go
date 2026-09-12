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

// client.go is the resource entity + lifecycle of this starter. The Milvus SDK
// is gRPC-based and accepts dial options, so the per-RPC resilience guard is
// installed as a client interceptor chain (see guard.go) — every RPC the
// wrapper's embedded client issues is protected without opt-in at the call
// site, matching the transparent per-request stance of the other NoSQL
// starters.
package StarterMilvus

import (
	"context"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// Client is the bean Milvus connections are injected as. It embeds the SDK's
// client.Client interface (so every method promotes unchanged) and holds the
// config plus the guard slot the interceptors consult.
type Client struct {
	client.Client
	cfg Config
	// slot is armed by Init; the gRPC interceptors read it on every RPC.
	slot *guardSlot
	// exec is the resilience executor, resolved via resilience.ExecutorFor;
	// no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("milvus:<addr>") exec scopes
	// limiter/breaker state by.
	resource string
}

// newClient builds the Milvus client — with the guard interceptors installed
// on the dial options — and probes it once so a wrong address or bad
// credential fails fast at startup instead of on first query.
func newClient(ctx context.Context, c Config) (*Client, error) {
	slot := &guardSlot{}
	cl, err := client.NewClient(ctx, client.Config{
		Address:     c.Addr,
		Username:    c.Username,
		Password:    c.Password,
		DBName:      c.Database,
		DialOptions: guardDialOptions(slot),
	})
	if err != nil {
		return nil, err
	}
	// Fail-fast probe: listing collections verifies reachability + auth.
	if _, err := cl.ListCollections(ctx); err != nil {
		_ = cl.Close()
		return nil, err
	}
	return &Client{Client: cl, cfg: c, slot: slot}, nil
}

// Init is the gs InitMethod: it resolves the executor through the neutral
// [resilience.ExecutorFor] seam (backed by starter-govern's governance center
// when imported), wraps it with the process-wide fault injector and
// observe-resilience, and arms the slot the interceptors read. When governance
// is off the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.resource = resilience.ResourceLabel("milvus", o.cfg.Addr)
	exec := fault.WrapExecutor(resilience.ExecutorFor("milvus", o.resource))
	o.exec = exec
	o.slot.arm(exec, o.resource)
	return nil
}

// Destroy closes the resilience executor (if armed) and the connection.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return o.Client.Close()
}

// Health reports whether Milvus answers a trivial query (list collections).
func (o *Client) Health(ctx context.Context) error {
	_, err := o.Client.ListCollections(ctx)
	return err
}
