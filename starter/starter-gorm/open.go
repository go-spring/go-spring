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

package gormcore

import (
	"fmt"

	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	gormobserve "go-spring.org/starter-gorm/observe"
	gormresilience "go-spring.org/starter-gorm/resilience"
	"gorm.io/gorm"
)

// Options carries the per-instance inputs a dialect starter hands to [Open]: the
// observability label, the teardown closers for driver-scoped state (discovery
// watches, registered TLS configs), and the container's facilities. The first
// group comes from the dialect's [Build] (via [Spec]); the last is filled by the
// module wiring, which is the only place the governance beans are reachable — a
// dialect never depends on cloud/governance.
type Options struct {
	Engine         string   // db.system + service label (e.g. "mysql", "postgresql")
	Service        string   // precomputed resilience.ServiceLabel for this instance
	ObserveEnabled bool     // per-instance kill switch for the gorm observe plugin
	Closers        []func() // teardown hooks run by Destroy before the pool closes

	// Params carries the container's facilities (see [cloud.ClientParams]); [Open]
	// applies it while assembling, so the returned DB is complete. It is one
	// struct rather than a parameter per capability so this bundle — which every
	// dialect's wiring fills — stays stable as capabilities are added. The zero
	// value is meaningful: a DB opened without a container (a hand-built one, an
	// example, a test) degrades to an observed-only, loudly-unmanaged executor
	// instead of silently running with no protection at all.
	Params cloud.ClientParams
}

// DB is the wrapper bean gorm clients are injected as, shared verbatim by every
// dialect starter. It embeds *gorm.DB so all gorm methods promote unchanged —
// gorm's whole API is the promoted surface, and the observe/resilience hooks are
// installed on the DB itself, so there is no per-method wrapper to hide the raw
// handle behind.
type DB struct {
	*gorm.DB

	engine         string
	serviceLabel   string
	observeEnabled bool
	closers        []func()
	exec           resilience.ClientExecutor // from Params.ExecutorFor; unmanaged (observe-only) when no container
}

// Open opens gorm with the given dialector, applies the pool settings and runs
// the registered customizers, installs the shared observe plugin and the
// governance-driven resilience stack, failing fast (and closing the pool) on any
// error, then returns the wrapped DB.
//
// Assembly completes here: the DB is fully observed AND governed on return.
// There is no Init hook and nothing patches the DB afterwards — governance is
// applied HERE from opt.Params rather than by the wiring, so a DB cannot
// exist half-assembled. The startup connectivity check is still a separate probe
// run by the wiring (the package [Ping]), after Open returns, so the client is
// complete before it is probed.
func Open(dialector gorm.Dialector, pool PoolConfig, opt Options) (*DB, error) {
	// The governance rule may size the pool too — the resource half of isolation,
	// next to the bulkhead's concurrency half — and it wins over the per-instance
	// key, so one place configures both halves. A pool is built once and belongs
	// to the driver, which is why this is read here rather than by the running
	// executor that every other policy field reaches.
	if n := opt.Params.PolicyFor(opt.Service).MaxConns; n > 0 {
		pool.MaxOpenConns = n
	}
	db, err := gorm.Open(dialector, GormConfig(pool))
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	if err := ApplyPool(db, pool); err != nil {
		_ = closeSQL(db)
		return nil, fmt.Errorf("gorm pool: %w", err)
	}
	if err := ApplyDBCustomizers(db); err != nil {
		_ = closeSQL(db)
		return nil, fmt.Errorf("gorm customizer: %w", err)
	}
	o := &DB{
		DB:             db,
		engine:         opt.Engine,
		serviceLabel:   opt.Service,
		observeEnabled: opt.ObserveEnabled,
		closers:        opt.Closers,
	}
	// The observe plugin is part of what the client IS, not a later assembly
	// step, so it is installed here rather than from a lifecycle hook. It MUST
	// run before the governance callbacks below: its Initialize captures the
	// dialect's own gorm:<op> processors, which the governance layer then
	// replaces — install it after and it would capture the wrappers instead of
	// the SQL builders.
	if o.observeEnabled {
		if err := o.DB.Use(gormobserve.NewPlugin(o.engine)); err != nil {
			_ = closeSQL(db)
			return nil, fmt.Errorf("gorm observe: %w", err)
		}
	}
	// Governance is applied in the constructor, not by the wiring, so the DB is
	// complete the moment Open returns. The executor comes from the one place
	// that composition lives ([cloud.ClientParams.ExecutorFor]): the governed
	// executor when the container is present, and the observed-only,
	// loudly-unmanaged one when it is not (opt.Params is the zero value). The
	// manager resolves the backing executor lazily, on each Execute, so this call
	// order relative to the center's wiring is irrelevant, and the fault
	// injector is nil-safe (a transparent pass-through with no injector).
	o.exec = opt.Params.ExecutorFor(o.engine, o.serviceLabel)
	if err := gormresilience.ApplyCallbacks(o.DB, o.exec, o.serviceLabel); err != nil {
		_ = closeSQL(db)
		return nil, fmt.Errorf("gorm resilience: %w", err)
	}
	return o, nil
}

// Destroy is the gs destroy method: closes the resilience executor (if applied),
// runs the driver-registered teardown closers (stopping discovery watches,
// deregistering TLS configs), then closes the underlying connection pool.
func (o *DB) Destroy() error {
	if o.exec != nil {
		_ = o.exec.Close()
	}
	for _, c := range o.closers {
		if c != nil {
			c()
		}
	}
	return closeSQL(o.DB)
}

// closeSQL closes the underlying connection pool. It is a no-op (returning nil)
// when db has no *sql.DB behind it.
func closeSQL(db *gorm.DB) error {
	if sqlDB, err := db.DB(); err == nil {
		return sqlDB.Close()
	}
	return nil
}
