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
)

// Client wraps *memcache.Client so every operation flows through the
// module-local observer (trace span + duration metric + access log). It
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

	// obs emits the per-operation span/metric/access-log triple; built by Init.
	obs *observer

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

// Init is the gs InitMethod. It builds the module-local observer (see
// observe.go) and resolves the executor through the neutral
// [resilience.ExecutorFor] seam (backed by starter-govern's governance center
// when imported), so this client neither injects nor names cloud/governance.
// The executor is wrapped with the process-wide fault injector
// ([fault.InjectorFor], nil-safe); when governance is off the resolved
// executor is a transparent no-op.
func (c *Client) Init() error {
	c.obs = newObserver()
	c.resource = resilience.ResourceLabel("memcached", c.name)
	exec := fault.WrapExecutor(resilience.ExecutorFor(c.resource))
	c.exec = resilience.WrapExecutor(exec, "memcached")
	return nil
}

// Destroy releases the resilience executor (if armed). The memcache client
// itself keeps a lazy connection pool with no Close method and discovery runs
// inside the backend (the loader has no resources), so only the executor's
// resources are released here. It is the gs destroy method.
func (c *Client) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return nil
}
