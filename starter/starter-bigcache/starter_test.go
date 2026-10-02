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
	"testing"

	"go-spring.org/cloud/cache"
	"go-spring.org/spring/gs"
)

// recordingDriver stands in for a company driver: it records that the instance
// was built through it rather than through the bundled fallback, then assembles
// as usual so the test can also use the resulting cache.
type recordingDriver struct{ calls int }

func (d *recordingDriver) CreateClient(ctx context.Context, name string, c Config) (*Cache, error) {
	d.calls++
	return DefaultDriver{}.CreateClient(ctx, name, c)
}

// TestInstanceWiring pins what one configured instance becomes: the starter's
// own wrapper bean under the entry's name, and the `cache.Cache` façade under
// `bigcache:<name>` — both of them usable, and no Driver bean in sight when the
// application provides none.
func TestInstanceWiring(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.bigcache.instances.hot.shards", "16")
	}).RunTest(t, func(ts *struct {
		Hot    *Cache       `autowire:"hot"`
		Facade *cache.Cache `autowire:"bigcache:hot"`
		Driver Driver       `autowire:"?"`
	}) {
		if ts.Hot == nil {
			t.Fatal("expected the instance bean under the entry's name")
		}
		if ts.Facade == nil {
			t.Fatal("expected the cache.Cache façade under bigcache:<name>")
		}
		if ts.Driver != nil {
			t.Fatal("no Driver bean is provided by default; newClient falls back to DefaultDriver internally")
		}

		if err := ts.Hot.Set("k", []byte("v")); err != nil {
			t.Fatalf("Set through the wrapper: %v", err)
		}
		b, err := ts.Facade.GetBytes(context.Background(), "k")
		if err != nil || string(b) != "v" {
			t.Fatalf("GetBytes through the façade = %q, %v; want \"v\", nil", b, err)
		}
	})
}

// TestProvidedDriverIsUsed pins that a Driver bean in the container replaces the
// bundled fallback: the instance is assembled through it.
func TestProvidedDriverIsUsed(t *testing.T) {
	d := &recordingDriver{}
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.bigcache.instances.hot.shards", "16")
		app.Provide(func() Driver { return d })
	}).RunTest(t, func(ts *struct {
		Hot *Cache `autowire:"hot"`
	}) {
		if ts.Hot == nil {
			t.Fatal("expected the instance bean")
		}
		if d.calls != 1 {
			t.Fatalf("CreateClient called %d times, want 1", d.calls)
		}
	})
}

// TestNamedDriverSelection pins the per-instance `${driver}` key: with more than
// one Driver bean in the container, the entry cites one by name and the instance
// is assembled through exactly that one.
func TestNamedDriverSelection(t *testing.T) {
	var plain, corp recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.bigcache.instances.hot.shards", "16")
		app.Property("spring.bigcache.instances.hot.driver", "corp")
		app.Provide(func() Driver { return &plain }).Name("plain")
		app.Provide(func() Driver { return &corp }).Name("corp")
	}).RunTest(t, func(ts *struct {
		Hot *Cache `autowire:"hot"`
	}) {
		if ts.Hot == nil {
			t.Fatal("expected the instance bean")
		}
		if corp.calls != 1 || plain.calls != 0 {
			t.Fatalf("named selection dispatched to corp=%d plain=%d, want 1 and 0", corp.calls, plain.calls)
		}
	})
}

// TestFamilyDefaultBucket pins the fallback every instance inherits: a key an
// entry does not define is read from spring.bigcache.default.* — here so that an
// entry carrying only a name is still fully configured.
func TestFamilyDefaultBucket(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.bigcache.default.shards", "16")
		app.Property("spring.bigcache.instances.hot.life-window", "1m")
	}).RunTest(t, func(ts *struct {
		Hot *Cache `autowire:"hot"`
	}) {
		if ts.Hot == nil {
			t.Fatal("expected the instance bean to be built from the inherited defaults")
		}
		if err := ts.Hot.Set("k", []byte("v")); err != nil {
			t.Fatalf("Set: %v", err)
		}
	})
}
