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

package StarterAnts

import (
	"context"
	"time"

	"github.com/panjf2000/ants/v2"
	"go-spring.org/stdlib/goutil"
)

// Ensure antsPool implements Pool.
var _ Pool = (*antsPool)(nil)

// antsPool wraps *ants.Pool to implement Pool.
type antsPool struct {
	pool *ants.Pool
}

func (p *antsPool) Submit(task func()) error { return p.pool.Submit(task) }
func (p *antsPool) Running() int             { return p.pool.Running() }
func (p *antsPool) Free() int                { return p.pool.Free() }
func (p *antsPool) Cap() int                 { return p.pool.Cap() }
func (p *antsPool) Waiting() int             { return p.pool.Waiting() }
func (p *antsPool) Release()                 { p.pool.Release() }

// Config defines an ants goroutine-pool configuration. ants is a purely
// in-process worker pool, so there is no address or connection to configure —
// only sizing and scheduling knobs.
type Config struct {
	// Size is the capacity of the pool, i.e. the maximum number of concurrent
	// workers. A value <= 0 means the pool is unbounded.
	Size int `value:"${size:=256}"`

	// ExpiryDuration is how long an idle worker may live before the periodic
	// purger reclaims it. Ignored when DisablePurge is true.
	ExpiryDuration time.Duration `value:"${expiry-duration:=1s}"`

	// PreAlloc pre-allocates memory for the worker queue when true, trading
	// startup cost for lower allocation churn under load.
	PreAlloc bool `value:"${pre-alloc:=false}"`

	// MaxBlockingTasks is the maximum number of tasks allowed to block waiting
	// for a free worker when the pool is full. 0 means no limit.
	MaxBlockingTasks int `value:"${max-blocking-tasks:=0}"`

	// Nonblocking makes Submit return ErrPoolOverload immediately instead of
	// blocking when the pool is full. When true, MaxBlockingTasks is ignored.
	Nonblocking bool `value:"${nonblocking:=false}"`

	// DisablePurge keeps workers alive forever, disabling the background purge
	// goroutine. Useful for pools that stay busy and want to avoid churn.
	DisablePurge bool `value:"${disable-purge:=false}"`
}

// ---------------------------------------------------------------------------
// Driver
// ---------------------------------------------------------------------------

// Driver defines how to create a Pool from configuration. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterAnts.Driver); when none is present,
// starter-ants falls back to the bundled [DefaultDriver] inside pool assembly.
// A custom driver is a bean, so it may inject the configuration/beans it needs
// — e.g. company config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every pool under
// ${spring.ants} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreatePool(c Config) (Pool, error)
}

// ---------------------------------------------------------------------------
// DefaultDriver
// ---------------------------------------------------------------------------

// DefaultDriver is the bundled default implementation of the Driver interface.
// It creates a standard *ants.Pool wrapped in an antsPool. Observer chaining is
// applied on top by the pool assembly (createPool), not by the driver.
type DefaultDriver struct{}

// CreatePool creates a new ants pool based on the provided configuration.
// A panicking task is bridged straight into the shared goutil panic chain
// (the structured-log bridge go-spring.org/log installs), so pool panics
// land in the same report stream as goroutine, handler and job panics. ants
// hands over only the panic value, so ReportPanic is called from the
// deferred recover in the worker — the panicking frames are still on the
// stack. A process needing custom panic handling contributes its own Driver
// bean with its own ants.WithPanicHandler.
func (DefaultDriver) CreatePool(c Config) (Pool, error) {
	pool, err := ants.NewPool(c.Size,
		ants.WithExpiryDuration(c.ExpiryDuration),
		ants.WithPreAlloc(c.PreAlloc),
		ants.WithMaxBlockingTasks(c.MaxBlockingTasks),
		ants.WithNonblocking(c.Nonblocking),
		ants.WithDisablePurge(c.DisablePurge),
		ants.WithPanicHandler(func(p any) {
			goutil.ReportPanic(context.Background(), p)
		}),
	)
	if err != nil {
		return nil, err
	}
	return &antsPool{
		pool: pool,
	}, nil
}
