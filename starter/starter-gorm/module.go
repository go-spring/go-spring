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
	"context"
	"time"

	"go-spring.org/cloud"
	"go-spring.org/stdlib/errutil"
	"gorm.io/gorm"
)

// Spec is the per-instance result of a dialect's Build: everything [NewDB] needs
// beyond the dialect's own literal labels. Closers are run on teardown (and on
// open failure) to release driver-scoped state such as discovery watches and
// registered TLS configs.
type Spec struct {
	Dialector      gorm.Dialector
	Pool           PoolConfig
	Service        string // governance service label (precomputed resilience.ServiceLabel)
	ObserveEnabled bool
	Closers        []func()
}

// NewDB assembles the client for one configured entry from the dialect's [Spec]:
// [Open] (which installs the observe plugin and the governance-driven resilience
// stack), the teardown closers on any failure, and the opt-in startup ping. It is
// the construction half of an entry; the registration half — the gs.Module, the
// per-entry *DB bean and its name, the paired health.Indicator and the
// ${discovery} binding — lives in each dialect starter, so a reader of that
// starter sees exactly what it registers.
//
// engine is the dialect's db.system / service label (e.g. "mysql",
// "microsoft.sql_server"); params carries the container's facilities (see
// [cloud.ClientParams]); ping runs the startup connectivity probe (the entry's
// ping key), whose window is spec.Pool.PingTimeout (0 = 5s).
func NewDB(ctx context.Context, engine string, spec Spec, params cloud.ClientParams, ping bool) (*DB, error) {
	// Assembly: open + pool + customizers + observe plugin + governance, all of
	// it in Open — the dialect Build never sees the governance authorities, so
	// the wiring bundles them into the Options it hands Open. Only then probe, so
	// the client is complete before it is checked, and a failure at any step
	// releases what was just assembled.
	db, err := Open(spec.Dialector, spec.Pool, Options{
		Engine:         engine,
		Service:        spec.Service,
		ObserveEnabled: spec.ObserveEnabled,
		Closers:        spec.Closers,
		Params:         params,
	})
	if err != nil {
		runClosers(spec.Closers)
		return nil, err
	}
	// Fail fast (opt-in, e.g. ping=true): probe the assembled DB with a ping at
	// startup so an unreachable database surfaces during boot rather than on the
	// first query. HealthCheck goes straight to the raw pool on purpose: it is a
	// connectivity check, not business traffic, so it must not open a span or
	// spend limiter/breaker budget. A failure abandons the DB, so release what
	// was just applied. With ping unset the probe is skipped and a database that
	// is not up yet only surfaces on first use.
	if ping {
		timeout := spec.Pool.PingTimeout
		if timeout <= 0 {
			timeout = 5 * time.Second
		}
		pctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		if err := HealthCheck(pctx, db); err != nil {
			_ = db.Destroy()
			return nil, errutil.Explain(err, "gorm: startup ping failed")
		}
	}
	return db, nil
}

// runClosers runs the driver-registered teardown hooks, tolerating nils, so an
// entry that fails to assemble releases exactly the driver-scoped state its
// dialect had already acquired (discovery watches, registered TLS configs).
func runClosers(closers []func()) {
	for _, closer := range closers {
		if closer != nil {
			closer()
		}
	}
}
