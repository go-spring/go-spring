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
// resource label, and the dynamicTransport indirection that lets Init swap the
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
	observe "go-spring.org/cloud/observe"
	"go-spring.org/cloud/observe/resilience"
)

// Client is the wrapper bean S3 clients are injected as. It embeds the
// concrete *minio.Client (so every generated method promotes unchanged) and
// field-injects the observability policy. newClient returns one; gs
// field-injects Observability, then calls Init (InitMethod) to build the
// observe transport + executor and swap them into the client's dynamic
// transport.
//
// The minio seam: the transport is fixed inside minio.Options at construction
// and cannot be swapped on the client afterwards. To arm protection after
// field injection, DefaultDriver installs a thin [dynamicTransport] (an
// atomic RoundTripper indirection) as the client's transport; Init then
// builds the observe+resilience transport and swaps it in.
type Client struct {
	*minio.Client
	// Observability is field-injected by gs from the top-level
	// "observability.*" keys and configures the observe transport (spans +
	// metrics + access log per request). The instance-prefixed
	// "spring.s3.<name>.observability.*" keys land in cfg.Observability and
	// take precedence — see resolveObservability.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// cfg is the connection config, retained for the resilience resource label.
	cfg Config
	// dyn is the dynamic transport DefaultDriver installed; Init swaps the
	// observe+resilience transport into it. nil for custom drivers.
	dyn *dynamicTransport
	// exec is the resilience executor protecting requests, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("s3:<endpoint>") exec scopes
	// limiter/breaker state by.
	resource string
}

// resolveObservability merges the two observability config surfaces into the
// effective policy Init arms:
//
//   - the instance-prefixed "spring.s3.<name>.observability.*" keys, bound
//     into Config (cfg.Observability) by conf.BindEach;
//   - the top-level "observability.*" keys, field-injected into the wrapper's
//     Observability field (an absolute property reference, kept for backward
//     compatibility with configs that predate the instance keys).
//
// Instance-level keys override top-level ones. Because binding fills the
// defaults ("brief" / 512 / no skips) even when no instance key is present, an
// instance value is only detectable as "set" when it differs from those
// defaults — setting an instance key back to its default value therefore
// cannot override a non-default top-level value. Configure one surface only
// and this caveat disappears.
func (o *Client) resolveObservability() observe.ObserveConfig {
	c := o.Observability // top-level fallback
	i := o.cfg.Observability
	if i.Level != "" && i.Level != observe.DefaultBrief {
		c.Level = i.Level
	}
	if i.MaxArgBytes != 0 && i.MaxArgBytes != 512 {
		c.MaxArgBytes = i.MaxArgBytes
	}
	if len(i.SkipOps) > 0 {
		c.SkipOps = i.SkipOps
	}
	return c
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It builds the observe transport (needs
// Observability) and resolves the executor through the neutral
// [resilience.ExecutorFor] seam (backed by starter-govern's governance center
// when imported), wraps it with the process-wide fault injector
// ([fault.InjectorFor], nil-safe), and swaps the result into the client's
// dynamic transport. When governance is off the resolved executor is a
// transparent no-op (the round-tripper is effectively observe-only).
func (o *Client) Init() error {
	// minio-go ships no OTel instrumentation of its own, so unlike
	// starter-elasticsearch the observe transport carries all three signals:
	// span + metric + access log.
	obsCfg := o.resolveObservability()
	obs := observe.NewDB("s3", obsCfg)
	observeTransport := &obsTransport{base: http.DefaultTransport, obs: obs}
	o.resource = resilience.ResourceLabel("s3", o.cfg.Endpoint)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	// Wrap the executor with observe-resilience so circuit-breaker trips,
	// rate-limit rejects, bulkhead rejections and retries emit a span + call
	// counter (by outcome) + duration histogram + access log.
	exec = resilobserve.WrapExecutor(exec, "s3", obsCfg)
	o.exec = exec
	if o.dyn != nil {
		o.dyn.Swap(resilience.NewRoundTripper(observeTransport, exec,
			func(*http.Request) string { return o.resource }))
	}
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
