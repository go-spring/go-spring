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
// wrapper S3 clients are injected as, its construction ([NewClient]) and
// teardown ([Client.Destroy]), the resilience service label, and the
// dynamicTransport indirection that lets the resilience transport
// be installed after the raw minio client — whose transport is fixed at
// construction — has been built. It mirrors starter-memcached's client.go. The
// per-request identity seam lives in command.go.

package StarterS3

import (
	"net/http"
	"sync"

	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/cloud/runtime/dataplane/label"

	"github.com/minio/minio-go/v7"
	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/resilience"
)

// Client is the wrapper bean S3 clients are injected as. It embeds the concrete
// *minio.Client, so every method of the raw client promotes unchanged and there
// is no per-method delegation to keep in step. The instrumentation does not
// live on the wrapper at all: the declaration+resilience transport is installed
// on the client's HTTP transport (command.go), which [NewClient] swaps in. The
// wrapper's whole job is thus to own that transport stack and the identity it is
// scoped by, so it stays a holder rather than a re-declaration of the S3 API.
//
// The type is exported because the declaration+resilience layers live on the
// client's HTTP transport and because apps inject *Client rather than
// *minio.Client so the assembly (identity + transport) is always this package's.
// [Driver.CreateClient] returns this type too, so a custom driver works with the
// same type the ecosystem sees.
type Client struct {
	// minio.Client is the raw client, embedded so its full S3 surface promotes
	// unchanged. Every promoted call still routes through the client's HTTP
	// transport, so declaration and governance apply uniformly with nothing to
	// re-declare here.
	*minio.Client

	// cfg is the connection config, retained for the resilience service label.
	cfg Config

	// dyn is the dynamic transport [DefaultDriver] installed as the client's
	// transport; [NewClient] swaps the declaration+resilience transport into it.
	// nil for a custom driver that built the raw client without the indirection —
	// resilience and declaration are then simply unavailable for that client.
	dyn *dynamicTransport

	// exec is the resilience executor protecting requests, built by [NewClient]
	// from the [runtime.Runtime] the container supplied; the center is required
	// to carry a [resilience.ClientManager] rather than a bare client.
	exec chain.Executor
	// serviceLabel is the resilience service key ("s3:<endpoint>") exec scopes
	// limiter/breaker state by.
	serviceLabel string
}

// NewClient builds a complete Client — identity, governance and all — over a
// connected raw client. client must be ready for use — it is normally the
// Driver's product — and transport must be the [dynamicTransport] that was
// handed to minio at construction time, so the declaration+resilience transport
// can be swapped into it; a custom driver that installed no indirection passes
// anything else (typically nil), and the client then runs with no declaration or
// resilience transport because its HTTP transport is not reachable.
//
// center carries the container's facilities (see [runtime.Runtime]), and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client must supply a [runtime.Runtime] over a
// [resilience.ClientManager]: [runtime.Runtime.ResilienceClient] is required, so a nil
// center is a construction error — never a graceful downgrade.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(client *minio.Client, transport http.RoundTripper, cfg Config, center *runtime.Runtime) *Client {
	c := &Client{
		Client: client,
		cfg:    cfg,
	}
	// Only the driver's own indirection is swappable; anything else is stored as
	// "no swap target", so the transport stack stays unreachable and the client
	// runs without declaration or governance.
	if t, ok := transport.(*dynamicTransport); ok {
		c.dyn = t
	}
	c.serviceLabel = label.ServiceLabel("s3", c.cfg.Endpoint)
	c.exec = center.ExecutorFor("s3", c.serviceLabel)
	c.installTransport()
	return c
}

// installTransport (re)installs the client's transport stack onto dyn: the
// resilience round-tripper, given [callFor] as its operation resolver so it
// builds each request's identity and passes it explicitly to the executor,
// wrapping http.DefaultTransport. The identity is built at the point the
// executor reads the operation ([chain.Executor.Execute] entry), so the emitted
// call covers every attempt, retries included — a declaration nested inside the
// executor would run per attempt. It is a no-op when dyn is nil: a custom driver
// that did not install the indirection has nothing to swap.
func (c *Client) installTransport() {
	if c.dyn == nil {
		return
	}
	c.dyn.Swap(resilience.NewRoundTripper(http.DefaultTransport, c.exec, c.serviceLabel, callFor))
}

// Destroy releases the resilience executor. The minio client holds no
// server-side session to close. It is the gs destroy method.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return nil
}

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction. minio fixes the transport at construction
// time, so to keep the wrapped transport installable after the raw client is
// built the fixed transport is this indirection and [Client.installTransport]
// swaps in the resilience transport (which builds each request's identity
// through [callFor]). Until then it passes straight
// through to http.DefaultTransport.
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
