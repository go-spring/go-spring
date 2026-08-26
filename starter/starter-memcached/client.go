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
// wrapper memcached clients are injected as, plus its lifecycle (Init/Destroy).
// It mirrors starter-redigo's pool.go. The per-operation command surface lives
// in command.go.
package StarterMemcached

import (
	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	observe "go-spring.org/cloud/observe"
	resilobserve "go-spring.org/cloud/observe/resilience"
)

// Client wraps *memcache.Client so every operation flows through the
// shared observe kit (trace span + duration/in-flight metric + access log). It
// embeds the real client, so methods not overridden in command.go (only Close, a
// lifecycle method) are promoted unchanged. memcache's API carries no context,
// so spans are root spans (not linked to the caller's request trace) — a
// limitation of gomemcache, documented here.
//
// The type is exported because gomemcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Client rather
// than *memcache.Client; the embedded field is available for any third-party
// API that needs the raw client.
type Client struct {
	*memcache.Client
	obs *observe.Observer

	// Both resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
	// governance center — so this struct has zero coupling to cloud/governance.
	Observability observe.ObserveConfig `value:"${observability:=}"`

	// name is the instance name (the spring.memcached.<name> map key), used for
	// the resilience resource label. Set by newClient; Init reads it.
	name string

	// exec is the resilience executor protecting every operation, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("memcached:<instance-name>")
	// exec scopes limiter/breaker state by.
	resource string
}

// Init is the gs InitMethod: gs field-injects Observability after newClient
// returns, then calls this. It builds the observe.Observer and resolves the
// executor through the neutral [resilience.ExecutorFor] seam (backed by
// starter-govern's governance center when imported), so this client neither
// injects nor names cloud/governance. The executor is wrapped with the
// process-wide fault injector ([fault.InjectorFor], nil-safe); when governance
// is off the resolved executor is a transparent no-op.
func (c *Client) Init() error {
	c.obs = observe.NewDB("memcached", c.Observability)
	c.resource = resilience.ResourceLabel("memcached", c.name)
	exec := fault.WrapExecutor(resilience.ExecutorFor(c.resource))
	c.exec = resilobserve.WrapExecutor(exec, "memcached", c.Observability)
	return nil
}

// Close releases the resilience executor (if armed) and stops any discovery
// Resolver watch behind the client. The memcache client itself keeps a lazy
// connection pool with no Close method, so only the background watch and the
// executor's resources are released here. It is the gs destroy method.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	stopLiveResolver(c.Client)
	return nil
}
