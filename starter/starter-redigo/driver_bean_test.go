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

package StarterRedigo

import (
	"context"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
)

// recordingDriver is a custom Driver that records each CreateClient call, so a
// test can assert createPool actually dispatched through it rather than the
// bundled DefaultDriver.
type recordingDriver struct {
	called bool
}

func (d *recordingDriver) CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*Pool, error) {
	d.called = true
	return NewPool(ctx, c, backend)
}

// TestDriverBeanDefault proves that when no company Driver bean is provided the
// pool still assembles — createPool falls back to the bundled DefaultDriver
// internally (no default bean is registered). The redigo pool dials lazily, so
// creating the pool bean (NewPool) needs no live redis.
func TestDriverBeanDefault(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.redigo.instances.cache.addr", "127.0.0.1:6379")
	}).RunTest(t, func(ts *struct {
		Pool   *Pool  `autowire:"cache"`
		Driver Driver `autowire:"?"`
	}) {
		if ts.Pool == nil {
			t.Fatal("expected a wired pool via the internal DefaultDriver fallback")
		}
		if ts.Driver != nil {
			t.Fatal("no Driver bean expected by default; redigo falls back internally")
		}
	})
}

// TestDriverBeanOverrideReachesPool proves an application-provided Driver bean
// is injected into createPool (replacing the internal DefaultDriver fallback)
// when a pool instance is assembled.
func TestDriverBeanOverrideReachesPool(t *testing.T) {
	driver := &recordingDriver{}
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.redigo.instances.cache.addr", "127.0.0.1:6379")
		app.Provide(func() Driver { return driver })
	}).RunTest(t, func(ts *struct {
		Pool *Pool `autowire:"cache"`
	}) {
		if ts.Pool == nil {
			t.Fatal("expected a wired pool")
		}
		if !driver.called {
			t.Fatal("expected createPool to dispatch through the overriding Driver bean")
		}
	})
}

// TestDriverBeanNamedSelection proves the per-instance ${driver} key selects a
// Driver bean by NAME: with two Driver beans in the container, the entry cites
// one and the pool is assembled through exactly that one.
func TestDriverBeanNamedSelection(t *testing.T) {
	var first, second recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.redigo.instances.cache.addr", "127.0.0.1:6379")
		app.Property("spring.redigo.instances.cache.driver", "corp")
		app.Provide(func() Driver { return &first }).Name("plain")
		app.Provide(func() Driver { return &second }).Name("corp")
	}).RunTest(t, func(ts *struct {
		Pool *Pool `autowire:"cache"`
	}) {
		if ts.Pool == nil {
			t.Fatal("expected a wired pool")
		}
		if first.called || !second.called {
			t.Fatalf("expected the pool to go through the named driver only: plain=%v corp=%v", first.called, second.called)
		}
	})
}
