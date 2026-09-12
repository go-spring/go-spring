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
// wrapper InfluxDB clients are injected as, plus its lifecycle (Init/
// Destroy), the resource label, and the dynamicTransport indirection that
// lets Init swap the observe+resilience transport into a client whose HTTP
// client is fixed at construction time. It mirrors starter-s3's client.go.
// The per-request observe seam lives in command.go.
package StarterInfluxdb

import (
	"context"
	"net/http"
	"sync"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
)

// Client is the wrapper bean InfluxDB clients are injected as. It embeds the
// concrete influxdb2.Client (so every generated method promotes unchanged).
// newClient returns one; gs calls Init (InitMethod) to build the observe
// transport + executor and swap them into the client's dynamic transport.
type Client struct {
	influxdb2.Client

	// cfg is the connection config, retained for the write helpers and the
	// resilience resource label.
	cfg Config
	// dyn is the dynamic transport DefaultDriver installed; Init swaps the
	// observe+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport
	// exec is the resilience executor protecting writes, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("influxdb:<url>") exec scopes
	// limiter/breaker state by.
	resource string
	// errDone tracks the async-writer error drain goroutine.
	errOnce sync.Once
}

// Init is the gs InitMethod: it builds the observe transport (influxdb-client-go
// ships no OTel instrumentation of its own, so the transport carries all three
// signals) and resolves the executor through the neutral
// [resilience.ExecutorFor] seam, wraps it with the process-wide fault injector
// and observe-resilience, and swaps the result into the client's dynamic
// transport. When governance is off the resolved executor is a transparent
// no-op (the transport is effectively observe-only).
func (o *Client) Init() error {
	obs := newDBObserver("influxdb")
	observeTransport := &obsTransport{base: http.DefaultTransport, obs: obs}
	o.resource = resilience.ResourceLabel("influxdb", o.cfg.ServerURL)
	exec := fault.WrapExecutor(resilience.ExecutorFor("influxdb", o.resource))
	o.exec = exec
	if o.dyn != nil {
		o.dyn.Swap(resilience.NewRoundTripper(observeTransport, exec,
			func(*http.Request) string { return o.resource }))
	}
	return nil
}

// Destroy is the gs destroy method: it closes the underlying client — which
// flushes and releases the async WriteAPI this wrapper handed out — then
// closes the resilience executor (if armed).
func (o *Client) Destroy() error {
	o.Client.Close()
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
	w := o.Client.WriteAPIBlocking(o.cfg.Org, o.cfg.Bucket)
	return o.exec.Execute(ctx, o.resource, func(ctx context.Context) error {
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
	w := o.Client.WriteAPI(o.cfg.Org, o.cfg.Bucket)
	o.errOnce.Do(func() {
		go func() {
			for err := range w.Errors() {
				if err == nil {
					continue
				}
				log.Errorf(context.Background(), starterTag, "influxdb: async write failed: %v", err)
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
func errMissingOrgBucket() error {
	return missingOrgBucket{}
}

type missingOrgBucket struct{}

func (missingOrgBucket) Error() string {
	return "influxdb: write helpers need org and bucket (set spring.influxdb.instances.<name>.org/.bucket)"
}

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction — the mechanism Init uses to arm observe+
// resilience on a client whose HTTP client is fixed at construction time.
// Until Init runs it passes straight through to http.DefaultTransport.
//
// The slot is guarded by a RWMutex rather than an atomic.Value because the
// active round-tripper can be any of several distinct concrete types
// (http.DefaultTransport, *obsTransport, the resilience round-tripper), and
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
