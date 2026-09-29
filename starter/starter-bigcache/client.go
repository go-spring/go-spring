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

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
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

	// mgr and inj are the governance beans gs injects into the constructor (both
	// nil in a standalone call). mgr is normalized in Init: an unarmed manager is
	// exactly the "governance off" pass-through, while a nil pointer would panic
	// on the method call. inj is nil-safe at its use site.
	mgr *resilience.Manager
	inj *fault.Injector

	// name is the instance name (the spring.bigcache.instances.<name> map key), used for
	// the resilience service label. Set by newClient; Init reads it.
	name string

	// exec is the resilience executor protecting Get/Set/Delete, resolved from the
	// injected manager; no-op when governance is off.
	exec resilience.ClientExecutor
	// service is the resilience service key ("bigcache:<instance-name>")
	// exec scopes limiter/breaker state by.
	service string
}

// Init is the gs InitMethod. It builds the per-operation observer (observe.go)
// and arms the executor from the injected governance manager. The manager's
// ClientExecutorFor resolves its backing executor lazily, on each Execute, so the
// arming order relative to starter-governance's wiring is irrelevant; the fault
// injector wraps it with inj, which is nil-safe (with no injector the fault layer
// is a transparent pass-through). When governance is off — an unarmed manager —
// the resolved executor is a transparent no-op.
func (c *Cache) Init() error {
	c.obs = newDBObserver()
	c.service = resilience.ServiceLabel("bigcache", c.name)
	if c.mgr == nil {
		c.mgr = resilience.NewManager()
	}
	c.exec = fault.WrapClientExecutor(c.mgr.ClientExecutorFor("bigcache", c.service), c.service, c.inj)
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
