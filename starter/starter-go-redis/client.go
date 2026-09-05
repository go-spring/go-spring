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

// client.go is the "resource entity" concept of this starter — the Client
// wrapper go-redis clients are injected as, plus its lifecycle (Init/Destroy)
// and resource label. It mirrors starter-redigo's pool.go: the entity embeds
// the concrete client and owns the resilience executor + the teardown closer
// the Driver handed back, while the per-command instrumentation layers live in
// command.go (starter-redigo's conn.go analog).
package StarterGoRedis

import (
	"io"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// Client is the wrapper bean go-redis clients are injected as. It
// embeds the concrete redis.UniversalClient (a *redis.Client or *redis.ClusterClient
// depending on mode, so methods promote unchanged).
// newClient returns one; gs then calls Init (InitMethod).
// Both resilience and fault are resolved through neutral seams
// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
// governance center — so this struct has zero coupling to cloud/governance.
type Client struct {
	redis.UniversalClient

	cfg      Config              // for resourceLabel (address fields)
	exec     resilience.Executor // resolved via resilience.ExecutorFor; no-op when governance is off
	resource string
	stop     io.Closer // driver-supplied teardown (e.g. discovery resolver watch)
}

// Init is the gs InitMethod. It resolves the executor through the neutral
// [resilience.ExecutorFor] seam (backed by starter-govern's governance center
// when imported), wraps it with the process-wide fault injector
// ([fault.InjectorFor], nil-safe), then the access log, and attaches the
// per-command hook so every command flows through it. When governance is off
// the resolved executor is a transparent no-op.
func (o *Client) Init() error {
	o.resource = resourceLabel(o.cfg)
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, "redis")
	o.exec = exec
	// Layer order (go-redis hooks are FIFO — first added is outermost):
	//
	//   redisotel (trace span + metrics) — added by instrument() in newClient, outermost
	//   observeHook (access log)         — added here, inside the span, outside the breaker
	//   resilienceHook (breaker/retry)   — added here, innermost
	//
	// This is the canonical order the whole client-starter family shares
	// (observe → resilience → inner): the access log wraps the resilient call so
	// one log line covers the whole retry loop, and it rides redisotel's span
	// context for trace_id correlation. observeHook is attached before
	// resilienceHook precisely so it sits outside the breaker.
	applyObservability(o.UniversalClient)
	o.AddHook(&resilienceHook{exec: exec, resource: o.resource})
	return nil
}

// Destroy is the gs destroy method: closes the resilience executor (if armed),
// stops any driver-supplied teardown (the discovery resolver watch, when armed),
// and closes the underlying client.
func (o *Client) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	_ = o.stop.Close()
	return o.UniversalClient.Close()
}

// resourceLabel derives a stable, human-readable resilience resource key for a
// client, so limiter and breaker state is scoped per Redis instance rather than
// per command. It falls back across the mode-specific address fields via the
// shared [resilience.ResourceLabel] helper.
func resourceLabel(c Config) string {
	first := ""
	if len(c.Addrs) > 0 {
		first = c.Addrs[0]
	}
	return resilience.ResourceLabel("redis", c.ServiceName, c.MasterName, c.Addr, first)
}
