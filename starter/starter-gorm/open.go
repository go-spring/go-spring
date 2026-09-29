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

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	gormobserve "go-spring.org/starter-gorm/observe"
	gormresilience "go-spring.org/starter-gorm/resilience"
	"gorm.io/gorm"
)

// Options carries the per-instance driver seams a dialect starter hands to
// [Open]: the observability label and the teardown closers for driver-scoped
// state (discovery watches, registered TLS configs).
type Options struct {
	Engine         string   // db.system + service label (e.g. "mysql", "postgresql")
	Service        string   // precomputed resilience.ServiceLabel for this instance
	ObserveEnabled bool     // per-instance kill switch for the gorm observe plugin
	Closers        []func() // teardown hooks run by Destroy before the pool closes

	// Mgr and Inj are the governance beans the gs module injects (both nil in a
	// standalone call) and hands over for Init to arm the executor with. They are
	// passed as arguments rather than carried on the dialect Spec so the Spec
	// stays a pure description of what the dialect built.
	Mgr *resilience.Manager
	Inj *fault.Injector
}

// DB is the wrapper bean gorm clients are injected as, shared verbatim by every
// dialect starter. It embeds *gorm.DB so all gorm methods promote unchanged.
type DB struct {
	*gorm.DB

	engine         string
	service        string
	observeEnabled bool
	closers        []func()
	mgr            *resilience.Manager       // injected governance manager
	inj            *fault.Injector           // injected fault injector
	exec           resilience.ClientExecutor // from mgr.ClientExecutorFor; no-op when governance is off
}

// Open opens gorm with the given dialector, applies the pool settings and runs
// the registered customizers, failing fast (and closing the pool) on any error,
// then returns the wrapped DB. The observe plugin + resilience callbacks are
// installed later by [DB.Init].
func Open(dialector gorm.Dialector, pool PoolConfig, opt Options) (*DB, error) {
	db, err := gorm.Open(dialector, GormConfig(pool))
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	if err := ApplyPool(db, pool); err != nil {
		_ = closeSQL(db)
		return nil, fmt.Errorf("gorm ping: %w", err)
	}
	if err := ApplyDBCustomizers(db); err != nil {
		_ = closeSQL(db)
		return nil, fmt.Errorf("gorm customizer: %w", err)
	}
	return &DB{
		DB:             db,
		engine:         opt.Engine,
		service:        opt.Service,
		observeEnabled: opt.ObserveEnabled,
		closers:        opt.Closers,
		mgr:            opt.Mgr,
		inj:            opt.Inj,
	}, nil
}

// Init is the gs InitMethod. It installs the shared gorm observe plugin, arms
// the resilience executor from the injected governance manager, and routes every
// gorm processor through it via [gormresilience.ApplyCallbacks]. The manager's
// ClientExecutorFor resolves its backing executor lazily, on each Execute, so the
// arming order relative to starter-governance's wiring is irrelevant; the fault
// injector wraps it with inj, which is nil-safe (with no injector the fault layer
// is a transparent pass-through). When governance is off — an unarmed manager —
// the resolved executor is a transparent no-op.
func (o *DB) Init() error {
	if o.observeEnabled {
		if err := o.DB.Use(gormobserve.NewPlugin(o.engine)); err != nil {
			return err
		}
	}
	if o.mgr == nil {
		o.mgr = resilience.NewManager()
	}
	o.exec = fault.WrapClientExecutor(o.mgr.ClientExecutorFor(o.engine, o.service), o.service, o.inj)
	if err := gormresilience.ApplyCallbacks(o.DB, o.exec, o.service); err != nil {
		return err
	}
	return nil
}

// Destroy is the gs destroy method: closes the resilience executor, runs any
// driver-registered teardown closers (stopping discovery watches, deregistering
// TLS configs), then closes the underlying connection pool.
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
