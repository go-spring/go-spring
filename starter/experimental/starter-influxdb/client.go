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
// wrapper InfluxDB clients are injected as, plus its lifecycle, the service
// label and the dynamicTransport indirection [NewClient] uses to install the
// declaration+resilience transport into a client whose HTTP client is fixed at
// construction time. It mirrors starter-s3's client.go. The transport-level
// declaration lives in observe.go.
package StarterInfluxdb

import (
	"context"
	"net/http"
	"sync"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
)

// Client is the wrapper bean InfluxDB clients are injected as. The raw
// influxdb2.Client is embedded, not held in an unexported field: the seam is
// installed on that client itself — the declaration+resilience transport is
// swapped into the RoundTripper its HTTP client carries — so there is nothing to
// intercept in the wrapper, and the SDK's whole interface is promoted as-is.
// [NewClient] remains the only constructor, so a client can never exist without
// its config. The wrapper adds only the org/bucket-scoped write helpers below.
type Client struct {
	// influxdb2.Client is the raw client, embedded so its whole interface is
	// promoted. The declaration and resilience layers ride the dynamic transport
	// its HTTP client was built over (see [dynamicTransport]), so nothing here
	// needs to re-declare the SDK's methods.
	influxdb2.Client

	// cfg is the connection config, retained for the write helpers and the
	// resilience service label.
	cfg Config

	// dyn is the dynamic transport the driver installed; [NewClient] swaps the
	// declaration+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport

	// exec is the resilience executor protecting writes, applied by [NewClient]
	// from the governance bundle; observes-only when governance is off.
	exec resilience.ClientExecutor
	// serviceLabel is the resilience service key ("influxdb:<url>") exec scopes
	// limiter/breaker state by.
	serviceLabel string

	// errOnce tracks the async-writer error drain goroutine.
	errOnce sync.Once
}

// NewClient builds a complete Client — config, identity and governance — over a
// raw influxdb2.Client. transport is the RoundTripper the raw client was built
// over — the driver's [dynamicTransport], so the declaration+resilience
// transport can be installed in place; a custom driver may pass nil, in which
// case the client runs with no declaration or resilience on the wire (its HTTP
// transport is not reachable and cannot be replaced), though its own write
// helpers still go through the executor.
//
// params carries the container's facilities (see [cloud.ClientParams]) and is
// applied HERE so a Client cannot exist half-assembled: there is no Init step,
// no later patching, and nothing the container has to remember to call. A
// hand-built client passes the zero [cloud.ClientParams]; its executor then
// degrades to resilience.Unmanaged — observed, with a one-time warning that no
// protection applies — rather than silently running bare.
//
// The declaring transport is OUTSIDE the resilience round-tripper, never its
// base: the emitter reads the Operation at Execute entry, so the declaration
// must run before the executor (see [declareTransport]). The chain installed is
//
//	dyn → declareTransport → resilience.NewRoundTripper → http.DefaultTransport
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewClient(raw influxdb2.Client, transport http.RoundTripper, cfg Config, params cloud.ClientParams) *Client {
	o := &Client{Client: raw, cfg: cfg}
	o.serviceLabel = resilience.ServiceLabel("influxdb", cfg.ServerURL)
	o.exec = params.ExecutorFor("influxdb", o.serviceLabel)
	// Only the driver's own indirection is swappable; anything else leaves the
	// declaration+resilience transport uninstalled, so the client keeps the
	// executor on its own write helpers but not on the raw transport.
	if t, ok := transport.(*dynamicTransport); ok {
		o.dyn = t
		o.dyn.Swap(&declareTransport{base: resilience.NewRoundTripper(http.DefaultTransport, o.exec)})
	}
	return o
}

// Destroy is the gs destroy method: it closes the underlying client — which
// flushes and releases the async WriteAPI this wrapper handed out — then closes
// the resilience executor (if governance was applied).
func (o *Client) Destroy() error {
	o.Close()
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return nil
}

// WritePoints writes points synchronously to the configured org/bucket,
// routed through the resilience executor. It is the protected write path:
// on rejection (rate-limit or open circuit) the write is never attempted.
// For high-throughput buffered writes prefer WriteAPI, whose background
// batches are intentionally unguarded.
func (o *Client) WritePoints(ctx context.Context, points ...*write.Point) error {
	if o.cfg.Org == "" || o.cfg.Bucket == "" {
		return errMissingOrgBucket()
	}
	w := o.WriteAPIBlocking(o.cfg.Org, o.cfg.Bucket)
	return o.exec.Execute(ctx, func(ctx context.Context) error {
		return w.WritePoint(ctx, points...)
	})
}

// ManagedWriteAPI returns the managed asynchronous writer for the configured
// org/bucket: points are buffered and flushed in batches on a background
// goroutine, and Destroy flushes whatever is pending. Async writes are not
// routed through the resilience executor (batching retries are the client's
// own); failed batches surface on the returned Errors() channel, which this
// wrapper drains into go-spring's log — an undrained channel would block the
// writer on the first failure.
//
// The name avoids shadowing the embedded influxdb2.Client.WriteAPI(org,
// bucket), which stays available for other org/bucket pairs.
func (o *Client) ManagedWriteAPI() api.WriteAPI {
	if o.cfg.Org == "" || o.cfg.Bucket == "" {
		panic(errMissingOrgBucket())
	}
	w := o.WriteAPI(o.cfg.Org, o.cfg.Bucket)
	o.errOnce.Do(func() {
		go func() {
			for err := range w.Errors() {
				if err == nil {
					continue
				}
				// A flushed batch has no request context and no span — the
				// client's own goroutine flushes it — so the operation is named
				// explicitly (see [asyncWriteFields]) to keep this line joinable
				// to the db.client.* records the synchronous path emits for the
				// same endpoint. This batch never crosses the resilience executor
				// (see the method doc), so the line is the only failure signal
				// the single emitter cannot produce for it.
				log.Error(context.Background(), accessTag, append(asyncWriteFields(),
					log.Err(err),
					log.Msg("influxdb: async write failed"))...)
			}
		}()
	})
	return w
}

// Org returns the configured default organization (for callers that need to
// build their own Query/Delete APIs against it).
func (o *Client) Org() string { return o.cfg.Org }

// Bucket returns the configured default destination bucket.
func (o *Client) Bucket() string { return o.cfg.Bucket }

// errMissingOrgBucket builds the shared org/bucket validation error.
func errMissingOrgBucket() error { return missingOrgBucket{} }

type missingOrgBucket struct{}

func (missingOrgBucket) Error() string {
	return "influxdb: write helpers need org and bucket (set spring.influxdb.instances.<name>.org/.bucket)"
}

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction — the mechanism [NewClient] uses to install
// declaration+resilience on a client whose HTTP client is fixed at construction
// time. Until then it passes straight through to http.DefaultTransport.
//
// The slot is guarded by a RWMutex rather than an atomic.Value because the
// active round-tripper can be any of several distinct concrete types
// (http.DefaultTransport, *declareTransport, the resilience round-tripper), and
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
