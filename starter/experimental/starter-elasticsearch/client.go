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
// (Init/Destroy), the service label, and the dynamicTransport indirection
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
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client is the wrapper bean Elasticsearch clients are injected
// as. It embeds the concrete *elasticsearch.Client (so every generated method
// promotes unchanged) and builds its resilience executor from the injected
// [resilience.Manager] and [fault.Injector] beans, so the policy comes from the
// governance document and hot-reloads inside the executor. newClient returns
// one; gs calls Init (InitMethod) to build the observe transport + executor and
// swap them into the client's dynamic transport.
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

	// cfg is the connection config, retained for the resilience service label.
	cfg Config
	// dyn is the dynamic transport DefaultDriver installed; Init
	// swaps the observe+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport
	// exec is the resilience executor protecting requests, built in Init from
	// mgr + inj; a pass-through when governance is off.
	exec resilience.ClientExecutor
	// service is the resilience service key ("elasticsearch:<...>") exec
	// scopes limiter/breaker state by.
	service string

	// mgr and inj are the governance beans gs injects into the constructor (both
	// nil in a standalone call). mgr is normalized in Init, since an unarmed
	// manager is exactly the "governance off" pass-through while a nil pointer
	// would panic on the method call; inj is nil-safe at its use site.
	mgr *resilience.Manager
	inj *fault.Injector
}

// Init is the gs InitMethod: it builds the observe transport, builds the
// resilience executor from the injected manager (a nil manager is normalized to
// an unarmed one), wraps it with the injected fault injector (nil-safe), and
// swaps the result into the client's dynamic transport. When governance is off
// the executor is a transparent no-op (the round-tripper is then observe-only).
func (o *Client) Init() error {
	obs := newDBObserver()
	observeTransport := &obsTransport{base: http.DefaultTransport, obs: obs}
	o.service = serviceLabel(o.cfg)
	exec := fault.WrapClientExecutor(o.mgr.ClientExecutorFor("elasticsearch", o.service), o.service, o.inj)
	o.exec = exec
	if o.dyn != nil {
		o.dyn.Swap(resilience.NewRoundTripper(observeTransport, exec))
	}
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if armed)
// and closes the underlying client. Discovery runs inside the backend (the
// loader has no resources), so nothing discovery-related is released here.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
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

// serviceLabel derives a stable, human-readable resilience service key for a
// client, so limiter and breaker state is scoped per Elasticsearch cluster
// rather than per request. It falls back across the address fields via the
// shared [resilience.ServiceLabel] helper.
func serviceLabel(c Config) string {
	first := ""
	if len(c.Addresses) > 0 {
		first = c.Addresses[0]
	}
	return resilience.ServiceLabel("elasticsearch", c.ServiceName, c.CloudID, first)
}
