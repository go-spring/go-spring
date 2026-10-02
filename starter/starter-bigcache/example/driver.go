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

package main

import (
	"context"
	"sync/atomic"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	StarterBigCache "go-spring.org/starter-bigcache"
	"go-spring.org/stdlib/errutil"
)

// removals counts what bigcache handed to the OnRemove hook; main.go asserts it
// moved, which is how the eviction feature knows the hook is really wired.
var removals atomic.Int64

// HookingBigCacheDriver is a custom Driver, registered as a Driver BEAN below and
// selected by spring.bigcache.default.driver=hook.
//
// It exists for one reason: the bundled DefaultDriver builds its bigcache.Config
// internally and binds only the knobs the starter declares, so a field it does
// not bind - OnRemove here, equally Hasher or Logger - is unreachable without a
// Driver of your own. That is what this seam is for.
//
// Since a Driver bean is process-wide, this one assembles every instance under
// ${spring.bigcache}, so it copies the knobs DefaultDriver copies and adds its
// own on top.
type HookingBigCacheDriver struct{}

// CreateClient builds one instance. It follows the Driver contract's error rule:
// the error it returns reaches the container unwrapped, so it names the stage and
// the instance itself.
func (HookingBigCacheDriver) CreateClient(ctx context.Context, name string,
	c StarterBigCache.Config) (*StarterBigCache.Cache, error) {

	log.Infof(ctx, log.TagAppDef, "HookingBigCacheDriver::CreateClient(%s)", name)

	conf := bigcache.DefaultConfig(c.LifeWindow)
	conf.Shards = c.Shards
	conf.CleanWindow = c.CleanWindow
	conf.MaxEntriesInWindow = c.MaxEntriesInWindow
	conf.MaxEntrySize = c.MaxEntrySize
	conf.HardMaxCacheSize = c.HardMaxCacheSize
	conf.StatsEnabled = c.StatsEnabled

	// The field the starter does not bind - the point of this file.
	conf.OnRemove = func(string, []byte) { removals.Add(1) }

	client, err := bigcache.New(ctx, conf)
	if err != nil {
		return nil, errutil.Explain(err, "bigcache: create instance %q", name)
	}

	// NewCache is the only way to build the wrapper: it fixes the instance's
	// identity and its observability in one step. The raw cache is running by now,
	// so it is this branch's to close if the wrapping fails.
	cache, err := StarterBigCache.NewCache(client, name)
	if err != nil {
		_ = client.Close()
		return nil, err
	}
	return cache, nil
}

func init() {
	// One Driver BEAN, so no gs.Module / conf.Bind is needed. Every instance under
	// ${spring.bigcache} is built through it; the starter falls back to its bundled
	// DefaultDriver only when no Driver bean is registered.
	gs.Provide(func() StarterBigCache.Driver {
		return HookingBigCacheDriver{}
	}).Name("hook").Caller(1)
}
