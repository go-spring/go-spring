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

// client.go is the "resource entity" concept of this starter: the Cache
// wrapper bigcache instances are injected as, plus its lifecycle (Init/Destroy).
// It mirrors starter-memcached's client.go. The per-operation command surface
// lives in command.go.
package StarterBigCache

import (
	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// --- per-operation observe wrapper -------------------------------------------
//
// Cache wraps *bigcache.BigCache so Get/Set/Delete flow through this starter's
// observe layer (trace span + duration metric + access log, see observe.go), in
// addition to the cache-stat gauges above. bigcache is an in-process heap cache
// with no network, so the spans are root spans (no caller context to link) and
// the durations are sub-microsecond - the value is per-key access visibility and
// a uniform signal vocabulary with the other client starters. It embeds the real
// client, so Reset/Stats/Len/Capacity/Close are promoted unchanged.
//
// The type is exported because bigcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Cache rather
// than *bigcache.BigCache; the embedded field is available for any third-party
// API that needs the raw client.

type Cache struct {
	*bigcache.BigCache

	// obs is this starter's per-operation observer (observe.go): client span,
	// db.client.operation.duration metric, access log.
	obs *dbObserver

	// Both resilience and fault are resolved through neutral seams
	// ([resilience.ExecutorFor] / [fault.InjectorFor]) backed by starter-govern's
	// governance center — so this struct has zero coupling to cloud/governance.

	// name is the instance name (the spring.bigcache.instances.<name> map key), used for
	// the resilience resource label. Set by newClient; Init reads it.
	name string

	// exec is the resilience executor protecting Get/Set/Delete, resolved via
	// resilience.ExecutorFor; no-op when governance is off.
	exec resilience.Executor
	// resource is the resilience resource key ("bigcache:<instance-name>")
	// exec scopes limiter/breaker state by.
	resource string
}

// Init is the gs InitMethod. It builds the per-operation observer (observe.go)
// and resolves the executor through the neutral [resilience.ExecutorFor] seam
// (backed by starter-govern's governance center when imported), so this cache
// neither injects nor names cloud/governance. The executor is wrapped with the
// process-wide fault injector ([fault.InjectorFor], nil-safe); when governance
// is off the resolved executor is a transparent no-op.
func (c *Cache) Init() error {
	c.obs = newDBObserver()
	c.resource = resilience.ResourceLabel("bigcache", c.name)
	c.exec = fault.WrapExecutor(resilience.ExecutorFor("bigcache", c.resource))
	return nil
}

// Close releases the resilience executor (if armed) and the underlying BigCache.
// It is the gs destroy method.
func (c *Cache) Destroy() error {
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return c.BigCache.Close()
}
