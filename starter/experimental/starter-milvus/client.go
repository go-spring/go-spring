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
	"go-spring.org/spring/gs"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client is the bean Milvus connections are injected as. It embeds the SDK's
// client.Client interface (so every method promotes unchanged) and holds the
// config plus the guard slot the interceptors consult.
type Client struct {
	client.Client
	cfg Config
	// slot is armed by Init; the gRPC interceptors read it on every RPC.
	slot *guardSlot
	// exec is the resilience executor built from the injected
	// resilience.Manager; no-op when governance is off.
	exec resilience.ClientExecutor
	// service is the resilience service key ("milvus:<addr>") exec scopes
	// limiter/breaker state by.
	service string
	// mgr and inj are the governance beans gs injects into the constructor
	// (both nil in a standalone call). mgr is normalized in Init, since an
	// unarmed manager is exactly the "governance off" pass-through while a nil
	// pointer would panic on the method call; inj is nil-safe at its use site.
	mgr *resilience.Manager
	inj *fault.Injector
}

// newClient builds the Milvus client — with the guard interceptors installed
// on the dial options — and probes it once so a wrong address or bad
// credential fails fast at startup instead of on first query. The governance
// beans (mgr, inj) are retained on the Client for Init to build the executor
// with; both are nil in a standalone, non-gs call.
//
// cp carries the application context gs injects into a constructor (a bare
// context.Context is not an injectable bean).
func newClient(cp *gs.ContextProvider, c Config, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	ctx := cp.Context
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
	return &Client{Client: cl, cfg: c, slot: slot, mgr: mgr, inj: inj}, nil
}

// Init is the gs InitMethod: it builds the executor from the injected
// resilience.Manager, wraps it with the fault injector and observe-resilience,
// and arms the slot the interceptors read. When governance is off the executor
// is a transparent no-op.
func (o *Client) Init() error {
	o.service = resilience.ServiceLabel("milvus", o.cfg.Addr)
	exec := fault.WrapClientExecutor(o.mgr.ClientExecutorFor("milvus", o.service), o.service, o.inj)
	o.exec = exec
	o.slot.arm(exec)
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
