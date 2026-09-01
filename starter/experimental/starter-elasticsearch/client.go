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
// wrapper Elasticsearch clients are injected as, plus its lifecycle
// (Init/Destroy), the resource label, and the dynamicTransport indirection
// that lets Init hot-swap the observe+resilience transport into a client whose
// transport is fixed at construction time. It mirrors starter-redigo's pool.go
// and starter-memcached's client.go. The per-command observe seam lives in
// command.go.
package StarterElasticsearch

import (
	"context"
	"net/http"
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
)

// Client is the wrapper bean Elasticsearch clients are injected
// as. It embeds the concrete *elasticsearch.Client (so every generated method
// promotes unchanged) and field-injects the resilience policy via gs.Dync so it
// hot-reloads on config change. newClient returns one; gs field-injects
// Resilience + Observability, then calls Init (InitMethod) to build
// the observe transport + executor and swap them into the client's dynamic
// transport.
//
// The elasticsearch seam: the transport is fixed inside elasticsearch.Config at
// construction and cannot be swapped on the client afterwards. To preserve a
// hot-reloadable resilience policy, DefaultDriver installs a thin
// [dynamicTransport] (an atomic RoundTripper indirection) as the client's
// transport. Init then builds the observe+resilience transport from
// the injected policy and swaps it into the dynamic transport — so the
// protection the client actually uses is dynamic even though the transport
// instance is not.
type Client struct {
	*elasticsearch.Client
	// Both resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
	// governance center — so this struct has zero coupling to cloud/governance.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// cfg is the connection config, retained for the resilience resource label.
	cfg Config
	// dyn is the dynamic transport DefaultDriver installed; Init
	// swaps the observe+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport
	// resolver is the discovery watch behind a service-name client (nil when
	// direct/mesh); Close stops it on shutdown.
	resolver *discovery.Resolver
	// exec is the resilience executor protecting requests, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("elasticsearch:<...>") exec
	// scopes limiter/breaker state by.
	resource string
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It builds the observe transport (needs Observability)
// and resolves the executor through the neutral [resilience.ExecutorFor] seam
// (backed by starter-govern's governance center when imported), wraps it with the
// process-wide fault injector ([fault.InjectorFor], nil-safe), and swaps the
// result into the client's dynamic transport. When governance is off the resolved
// executor is a transparent no-op (the round-tripper is effectively observe-only).
func (o *Client) Init() error {
	obs := observe.NewDB("elasticsearch", o.Observability, observe.WithoutTrace())
	observeTransport := &obsTransport{base: http.DefaultTransport, obs: obs}
	o.resource = resourceLabel(o.cfg)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	// Wrap the executor with observe-resilience so circuit-breaker trips,
	// rate-limit rejects, bulkhead rejections and retries emit a span + call
	// counter (by outcome) + duration histogram + access log.
	exec = resilience.WrapExecutor(exec, "elasticsearch", o.Observability)
	o.exec = exec
	if o.dyn != nil {
		o.dyn.Swap(resilience.NewRoundTripper(observeTransport, exec,
			func(*http.Request) string { return o.resource }))
	}
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if armed),
// stops any discovery watch, and closes the underlying client.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	stopLiveResolver(o.resolver)
	return o.Client.Close(context.Background())
}

// dynamicTransports tracks the dynamic transport DefaultDriver installed for
// each client, so newClient can hand it to the wrapper for Init to
// arm. The key is the *elasticsearch.Client value; only clients built by
// DefaultDriver appear here.
var dynamicTransports sync.Map // *elasticsearch.Client -> *dynamicTransport

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction. elasticsearch fixes the transport at
// construction time, so to keep resilience hot-reloadable the fixed transport
// is this indirection and Init swaps in the observe+resilience
// transport (or the observe-only transport when resilience is disabled). Until
// Init runs it passes straight through to http.DefaultTransport.
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

// resourceLabel derives a stable, human-readable resilience resource key for a
// client, so limiter and breaker state is scoped per Elasticsearch cluster
// rather than per request. It falls back across the address fields via the
// shared [resilience.ResourceLabel] helper.
func resourceLabel(c Config) string {
	first := ""
	if len(c.Addresses) > 0 {
		first = c.Addresses[0]
	}
	return resilience.ResourceLabel("elasticsearch", c.ServiceName, c.CloudID, first)
}
