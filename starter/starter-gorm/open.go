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
	observe "go-spring.org/cloud/observe"
	gormobserve "go-spring.org/starter-gorm/observe"
	gormresilience "go-spring.org/starter-gorm/resilience"
	"gorm.io/gorm"
)

// Options carries the per-instance driver seams a dialect starter hands to
// [Open]: the observability labels and the teardown closers for driver-scoped
// state (discovery watches, registered TLS configs).
type Options struct {
	Engine         string   // db.system + observe + resource label (e.g. "mysql", "postgresql")
	Resource       string   // precomputed resilience.ResourceLabel for this instance
	ObserveEnabled bool     // per-instance kill switch for the gorm observe plugin
	Closers        []func() // teardown hooks run by Destroy before the pool closes
}

// DB is the wrapper bean gorm clients are injected as. It embeds *gorm.DB so all
// gorm methods promote unchanged, and field-injects Observability. A dialect
// starter re-exports it (`type DB = gormcore.DB`) so its own package still
// exposes a DB type while the wrapper body, lifecycle and observe/resilience
// wiring are shared here.
type DB struct {
	*gorm.DB
	Observability observe.ObserveConfig `value:"${observability:=}"`

	engine         string
	resource       string
	observeEnabled bool
	closers        []func()
	exec           resilience.Executor // resolved via resilience.ExecutorFor; no-op when governance is off
}

// Open opens gorm with the given dialector, applies the pool settings and runs
// the registered customizers, failing fast (and closing the pool) on any error,
// then returns the wrapped DB. The observe plugin + resilience callbacks are
// installed later by [DB.Init], after gs field-injects Observability.
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
		resource:       opt.Resource,
		observeEnabled: opt.ObserveEnabled,
		closers:        opt.Closers,
	}, nil
}

// Init is the gs InitMethod (runs after gs field-injects Observability). It
// installs the shared gorm observe plugin, resolves the resilience executor
// through the neutral [resilience.ExecutorFor] seam (backed by starter-govern's
// governance center when configured; a transparent no-op otherwise), wraps it
// with the process-wide fault injector ([fault.InjectorFor], nil-safe), and
// routes every gorm processor through it via [gormresilience.ApplyCallbacks].
func (o *DB) Init() error {
	if o.observeEnabled {
		if err := o.DB.Use(gormobserve.NewPlugin(o.engine, o.Observability)); err != nil {
			return err
		}
	}
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))
	exec = resilience.WrapExecutor(exec, o.engine, o.Observability)
	o.exec = exec
	if err := gormresilience.ApplyCallbacks(o.DB, exec, o.resource); err != nil {
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
