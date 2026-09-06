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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + DefaultDriver owns full client assembly. The Driver is an OPTIONAL
// container bean: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterBigCache.Driver); when none is present,
// starter-bigcache falls back to the bundled [DefaultDriver] inside client
// assembly. Because a custom driver is a bean, it may inject the configuration
// it needs at wiring time.
package StarterBigCache

import (
	"context"

	"github.com/allegro/bigcache/v3"
)

// Driver interface defines how to create a BigCache instance.
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*bigcache.BigCache, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new BigCache instance based on the provided configuration.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*bigcache.BigCache, error) {
	conf := bigcache.DefaultConfig(c.LifeWindow)
	conf.Shards = c.Shards
	conf.CleanWindow = c.CleanWindow
	conf.MaxEntriesInWindow = c.MaxEntriesInWindow
	conf.MaxEntrySize = c.MaxEntrySize
	conf.HardMaxCacheSize = c.HardMaxCacheSize
	conf.StatsEnabled = c.StatsEnabled
	return bigcache.New(ctx, conf)
}
