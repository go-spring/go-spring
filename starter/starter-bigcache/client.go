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
	"go.opentelemetry.io/otel/metric"

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
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

	// obs is this starter's shared instrument set (observe.go): the client span,
	// the db.client.operation.duration metric, the access log, and the
	// cache-statistics gauges. One per process - every Cache holds the same one.
	obs *dbObserver

	// gaugeRegs is this cache's own registration of its statistics against the
	// shared gauges. It is held here because the values those gauges report come
	// from this cache: the registration's lifetime is the cache's lifetime, and
	// Destroy takes it away. Dropping it would leave the instrument reporting a
	// destroyed cache, and holding the registration would pin it.
	gaugeRegs []metric.Registration

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
	c.obs = instruments()
	c.service = resilience.ServiceLabel("bigcache", c.name)
	c.exec = fault.WrapClientExecutor(c.mgr.ClientExecutorFor("bigcache", c.service), c.service, c.inj)

	// Register this cache's statistics against the shared gauges, labeled with
	// this instance's name so several caches in one process stay
	// distinguishable. A registration error is dropped rather than failing the
	// cache's startup: with no OTel SDK installed the meter is a no-op and there
	// is nothing to report, which is not a reason to refuse to serve.
	if reg, err := c.obs.observeGauges(c.BigCache, c.name); err == nil {
		c.gaugeRegs = append(c.gaugeRegs, reg)
	}
	return nil
}

// Destroy takes away this cache's gauge registration, releases the resilience
// executor (if armed), and closes the underlying BigCache. It is the gs destroy
// method.
//
// The unregistration comes first: once the cache is closed its statistics are
// meaningless, and a registration left behind would both report a dead cache and
// pin it. It is also why the registration lives on the cache rather than in the
// shared instrument set - the same reason the gauges are not registered by a
// creation-time callback.
func (c *Cache) Destroy() error {
	for _, reg := range c.gaugeRegs {
		_ = reg.Unregister()
	}
	c.gaugeRegs = nil
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return c.BigCache.Close()
}
