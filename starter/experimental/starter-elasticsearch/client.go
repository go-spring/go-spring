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

// client.go is the "resource entity" concept of this starter: the Client
// wrapper Elasticsearch clients are injected as, its construction ([NewClient])
// and teardown ([Client.Destroy]), the service label, and the
// dynamicTransport indirection that lets the resilience transport
// be installed after the raw client — whose transport is fixed at construction
// — has been built. It mirrors starter-memcached's client.go. The per-request
// declaration seam lives in command.go.

package StarterElasticsearch

import (
	"context"
	"go-spring.org/cloud/chain"
	"net/http"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/resilience"
)

// Client is the wrapper bean Elasticsearch clients are injected as. It embeds
// the concrete *elasticsearch.Client, so its whole surface promotes unchanged —
// both the lifecycle/administrative methods (Close, Perform, Metrics, ...) and
// the API tree. There is no per-method delegation to keep in step: the request
// naming does not live on the wrapper at all, but on the client's HTTP transport
// (command.go), which [NewClient] swaps in. The wrapper's whole job is
// thus to own that transport stack and the
// identity it is scoped by, so it stays a holder rather than a re-declaration of
// the client API.
//
// The API tree (esapi.API) reaches apps through that same promotion. Every
// Elasticsearch operation (Index, Get, Search, Bulk, the Cat/Indices/...
// subtrees, ...) is a func-valued FIELD of that tree, not a method —
// `es.Index.WithDocumentID(...)` only works because Index is a field carrying a
// method-bearing func type. Embedding *elasticsearch.Client promotes it whole;
// there is no separate embedded *esapi.API, which would sit at a shallower depth
// and shadow the promoted tree. Its members route through the client's HTTP
// transport, so promotion exposes no path around the governance (and request
// naming) layers.
//
// The type is exported because the resilience layer lives on the client's
// HTTP transport and because the API tree is the app-facing surface; apps inject
// *Client rather than *elasticsearch.Client. [Driver.CreateClient] returns this
// type too, so a custom driver works with the same type the ecosystem sees.
type Client struct {
	// elasticsearch.Client is the raw client, embedded so its full surface
	// promotes unchanged — the lifecycle/administrative methods and the API tree
	// alike. Every promoted call still routes through the client's HTTP
	// transport, so observation and governance apply uniformly with nothing to
	// re-declare here.
	*elasticsearch.Client

	// cfg is the connection config, retained for the resilience service label.
	cfg Config

	// dyn is the dynamic transport [DefaultDriver] installed as the client's
	// transport; [NewClient] swaps the resilience transport into it.
	// nil for a custom driver that built the raw client without the indirection.
	dyn *dynamicTransport

	// exec is the resilience executor protecting requests, set by [NewClient] from
	// the governance center it is handed. The center is required to carry a
	// [resilience.ClientManager]: [governance.Center.ExecutorFor] panics on a center
	// without one rather than leaving the round-tripper unprotected.
	exec chain.Executor
	// serviceLabel is the resilience service key ("elasticsearch:<...>") exec
	// scopes limiter/breaker state by.
	serviceLabel string
}

// NewClient builds a Client over a connected raw client, fixing its identity,
// governance and transport stack. client must be ready for use — it is normally
// the Driver's product — and transport must be the [dynamicTransport] that was
// handed to elasticsearch.Config at construction time, so the
// resilience transport can be swapped into it; a custom driver that
// installed no indirection passes anything else (typically nil).
//
// center carries the container's facilities (see [governance.Center]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client must supply a center holding a [resilience.ClientManager]:
// [governance.Center.ExecutorFor] is required, so a nil center
// is a construction error — [governance.Center.ExecutorFor] panics — never a
// graceful downgrade to an unprotected client.
//
// The service label falls back across the address fields via the shared
// [resilience.ServiceLabel] helper, so limiter and breaker state is scoped per
// Elasticsearch cluster rather than per request. The manager's ClientExecutorFor
// resolves its backing executor lazily, on each Execute, so the call order
// relative to the center's wiring is irrelevant.
func NewClient(client *elasticsearch.Client, transport http.RoundTripper, cfg Config, center *governance.Center) *Client {
	c := &Client{
		Client: client,
		cfg:    cfg,
	}
	// Only the driver's own indirection is swappable; anything else is stored as
	// "no transport stack", so no swap is installed.
	if t, ok := transport.(*dynamicTransport); ok {
		c.dyn = t
	}
	first := ""
	if len(cfg.Addresses) > 0 {
		first = cfg.Addresses[0]
	}
	c.serviceLabel = resilience.ServiceLabel("elasticsearch", cfg.ServiceName, cfg.CloudID, first)
	c.exec = center.ExecutorFor("elasticsearch", c.serviceLabel)
	c.installTransport()
	return c
}

// installTransport (re)installs the client's transport stack onto dyn: the
// resilience round-tripper wraps http.DefaultTransport, and it derives each
// request's operation itself through [callFor] — passed as the callFor callback,
// so the operation the executor reads at Execute entry is declared from the
// request, not carried on the context. It is a no-op when dyn is nil: a custom
// driver that did not install the indirection has nothing to swap.
func (c *Client) installTransport() {
	if c.dyn == nil {
		return
	}
	rt := resilience.NewRoundTripper(http.DefaultTransport, c.exec, c.serviceLabel, callFor)
	c.dyn.Swap(rt)
}

// Destroy releases the resilience executor and closes the underlying client.
// Discovery runs inside the backend (the loader has no resources), so nothing
// discovery-related is released here. It is the gs destroy method.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return c.Client.Close(context.Background())
}

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction. elasticsearch fixes the transport at
// construction time, so to keep resilience hot-swappable the fixed transport is
// this indirection and [Client.installTransport] swaps in the
// resilience transport. Until then it passes straight through to
// http.DefaultTransport.
//
// The slot is guarded by a RWMutex rather than an atomic.Value because the
// active round-tripper can be any of several distinct concrete types
// (http.DefaultTransport, the resilience round-tripper), and
// atomic.Value requires every stored value to have the same concrete type.
type dynamicTransport struct {
	mu  sync.RWMutex
	cur http.RoundTripper
}

func newDynamicTransport() *dynamicTransport {
	return &dynamicTransport{cur: http.DefaultTransport}
}

// Swap atomically replaces the active round-tripper.
func (t *dynamicTransport) Swap(rt http.RoundTripper) {
	t.mu.Lock()
	t.cur = rt
	t.mu.Unlock()
}

// RoundTrip delegates to the currently-active round-tripper.
func (t *dynamicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.RLock()
	rt := t.cur
	t.mu.RUnlock()
	return rt.RoundTrip(req)
}
