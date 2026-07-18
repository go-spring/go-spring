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
	"time"

	"github.com/panjf2000/ants/v2"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

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

	// Driver specifies which pool driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// Driver interface defines how to create an ants pool.
type Driver interface {
	CreatePool(c Config) (*ants.Pool, error)
}

// RegisterDriver registers an ants pool driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("ants driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreatePool creates a new ants pool based on the provided configuration.
func (DefaultDriver) CreatePool(c Config) (*ants.Pool, error) {
	return ants.NewPool(c.Size,
		ants.WithExpiryDuration(c.ExpiryDuration),
		ants.WithPreAlloc(c.PreAlloc),
		ants.WithMaxBlockingTasks(c.MaxBlockingTasks),
		ants.WithNonblocking(c.Nonblocking),
		ants.WithDisablePurge(c.DisablePurge),
	)
}
