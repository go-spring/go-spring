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

package StarterBigCache

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines BigCache in-memory cache configuration. BigCache is a purely
// in-process cache, so there is no address or connection pool to configure —
// only sizing and eviction knobs.
type Config struct {
	// Shards is the number of cache shards, must be a power of two.
	Shards int `value:"${shards:=1024}"`

	// LifeWindow is how long an entry may live before it is considered stale.
	LifeWindow time.Duration `value:"${life-window:=10m}"`

	// CleanWindow is the interval between background evictions of stale entries.
	// 0 disables the background cleaner.
	CleanWindow time.Duration `value:"${clean-window:=1m}"`

	// MaxEntriesInWindow is the expected number of entries within LifeWindow,
	// used only to pre-allocate memory at startup.
	MaxEntriesInWindow int `value:"${max-entries-in-window:=600000}"`

	// MaxEntrySize is the expected maximum size of a single entry in bytes,
	// used only to pre-allocate memory at startup.
	MaxEntrySize int `value:"${max-entry-size:=500}"`

	// HardMaxCacheSize is the hard memory limit in MB, 0 means unlimited.
	HardMaxCacheSize int `value:"${hard-max-cache-size:=0}"`

	// StatsEnabled records per-key hit/miss statistics when true.
	StatsEnabled bool `value:"${stats-enabled:=false}"`

	// Driver specifies which BigCache driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// Driver interface defines how to create a BigCache instance.
type Driver interface {
	CreateClient(c Config) (*bigcache.BigCache, error)
}

// RegisterDriver registers a BigCache driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("bigcache driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new BigCache instance based on the provided configuration.
func (DefaultDriver) CreateClient(c Config) (*bigcache.BigCache, error) {
	conf := bigcache.DefaultConfig(c.LifeWindow)
	conf.Shards = c.Shards
	conf.CleanWindow = c.CleanWindow
	conf.MaxEntriesInWindow = c.MaxEntriesInWindow
	conf.MaxEntrySize = c.MaxEntrySize
	conf.HardMaxCacheSize = c.HardMaxCacheSize
	conf.StatsEnabled = c.StatsEnabled
	return bigcache.New(context.Background(), conf)
}
