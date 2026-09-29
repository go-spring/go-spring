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
// wrapper S3 clients are injected as, plus its lifecycle (Init/Destroy), the
// service label, and the dynamicTransport indirection that lets Init swap the
// observe+resilience transport into a client whose transport is fixed at
// construction time. It mirrors starter-elasticsearch's client.go. The
// per-request observe seam lives in command.go.
package StarterS3

import (
	"net/http"
	"sync"

	"github.com/minio/minio-go/v7"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// Client is the wrapper bean S3 clients are injected as. It embeds the
// concrete *minio.Client (so every generated method promotes unchanged) and
// field-injects the observability policy. newClient returns one, arms its
// governance stack through [Client.ArmGovernance], and lets gs call Init
// (InitMethod) to build the observe transport and install it.
//
// The minio seam: the transport is fixed inside minio.Options at construction
// and cannot be swapped on the client afterwards. To arm the observe+resilience
// transport after field injection, DefaultDriver installs a thin
// [dynamicTransport] (an atomic RoundTripper indirection) as the client's
// transport; Init then builds the transport and swaps it in.
type Client struct {
	*minio.Client

	// cfg is the connection config, retained for the resilience service label.
	cfg Config
	// dyn is the dynamic transport DefaultDriver installed; Init swaps the
	// observe+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport
	// exec is the resilience executor protecting requests, armed by
	// ArmGovernance; nil means "governance off" — the round-tripper is
	// observe-only.
	exec resilience.ClientExecutor
	// service is the resilience service key ("s3:<endpoint>") exec scopes
	// limiter/breaker state by.
	service string
}

// Init is the gs InitMethod: it builds the observe transport and installs the
// observe+resilience round-tripper on the client's dynamic transport.
// Governance (the resilience executor) is armed separately by
// [Client.ArmGovernance], which the gs wiring calls with the injected beans —
// see that method for why it is not part of this lifecycle hook.
func (o *Client) Init() error {
	// minio-go ships no OTel instrumentation of its own, so unlike
	// starter-elasticsearch the observe transport carries all three signals:
	// span + metric + access log (see observe.go).
	obs := newDBObserver("s3")
	observeTransport := &obsTransport{base: http.DefaultTransport, obs: obs}
	if o.dyn != nil {
		o.dyn.Swap(resilience.NewRoundTripper(observeTransport, o.exec))
	}
	return nil
}

// ArmGovernance arms the governance-driven resilience stack. It is called by
// the gs wiring with the injected beans — nil when the container has no
// starter-governance, and nil from a standalone caller, both of which mean
// "governance off".
//
// The stack is fault( execFor ): fault wraps the resolved executor's operation
// fn so injected failures land INSIDE the retry/breaker loop, and
// [resilience.NewRoundTripper] sits outermost around the observe transport
// [Client.Init] builds. inj is nil-safe: with no injector (governance off /
// fault disabled) WrapClientExecutor returns the inner executor unchanged, so the
// fault layer is a transparent pass-through. Resolution is deferred to call
// time, so the order of this arming relative to starter-governance's wiring is
// irrelevant.
//
// The service label is computed here because the executor this builds is bound
// to it, and a retry or a breaker trip is reported under it. When governance is
// off there is nothing to bind: [resilience.NewRoundTripper] returns the observe
// transport unchanged on a nil executor, so every request is still observed.
func (o *Client) ArmGovernance(mgr *resilience.Manager, inj *fault.Injector) error {
	// A nil manager is the unwired case — a container without
	// starter-governance (the wiring injects it nullably, so it is nil there
	// too), or a standalone caller that built the client itself. A fresh
	// unarmed manager is exactly
	// "governance off": every resolve is an observe-only pass-through.
	// Normalizing here keeps the rest of this method (and every caller) free of
	// nil branches.
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	o.service = resilience.ServiceLabel("s3", o.cfg.Endpoint)
	o.exec = fault.WrapClientExecutor(mgr.ClientExecutorFor("s3", o.service), o.service, inj)
	return nil
}

// Destroy is the gs destroy method: it closes the resilience executor (if
// armed). The minio client holds no server-side session to close.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	return nil
}

// dynamicTransport is a thin http.RoundTripper indirection whose behavior can
// be swapped after construction. minio fixes the transport at construction
// time, so to keep the wrapped transport installable after field injection the
// fixed transport is this indirection and Init swaps in the observe+resilience
// transport. Until Init runs it passes straight through to
// http.DefaultTransport.
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
