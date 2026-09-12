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

package StarterHTTPClient

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/httpclt"
)

// TestDefaultDriverFallback proves assembleTransport falls back to the bundled
// DefaultDriver when no Driver bean is provided (nil). Building the transport
// needs no live server — it only assembles the RoundTripper.
func TestDefaultDriverFallback(t *testing.T) {
	rt, _, err := assembleTransport(nil, "x", Config{Addr: "10.0.0.1:8080"}, nil, nil)
	if err != nil {
		t.Fatalf("assembleTransport with nil driver failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected a transport from the DefaultDriver fallback")
	}
}

// recordingDriver is the "ADD to the default" shape; it delegates to
// DefaultDriver and records that it was the active assembly driver.
type recordingDriver struct {
	DefaultDriver
	called bool
	// backend records the discovery backend handed to CreateTransport, so a test
	// can prove the starter wiring resolves the ${discovery} label and passes it
	// to a custom driver (rather than keeping it private to DefaultDriver).
	backend discovery.Discovery
}

func (d *recordingDriver) CreateTransport(ctx context.Context, name string, c Config, backend discovery.Discovery) (http.RoundTripper, func() error, error) {
	d.called = true
	d.backend = backend
	return d.DefaultDriver.CreateTransport(ctx, name, c, backend)
}

// TestCustomDriverUsed proves a provided Driver bean is the one assembleTransport
// dispatches through, replacing the internal DefaultDriver fallback.
func TestCustomDriverUsed(t *testing.T) {
	drv := &recordingDriver{}
	rt, _, err := assembleTransport(nil, "x", Config{Addr: "10.0.0.1:8080"}, nil, drv)
	if err != nil {
		t.Fatalf("assembleTransport failed: %v", err)
	}
	if rt == nil {
		t.Fatal("expected a transport")
	}
	if !drv.called {
		t.Fatal("expected assembleTransport to dispatch through the provided Driver bean")
	}
}

// stubDiscovery is a minimal named discovery backend bean for wiring tests.
type stubDiscovery struct{}

func (stubDiscovery) Resolve(context.Context, string, ...discovery.Option) ([]discovery.Endpoint, error) {
	return nil, nil
}

// TestCustomDriverReceivesResolvedBackend proves the ${discovery} label is
// resolved by the starter wiring and handed to a custom Driver as an argument:
// the backend bean is reachable from outside StarterHTTPClient, not hidden in a
// private field of Config.
func TestCustomDriverReceivesResolvedBackend(t *testing.T) {
	nacos, consul := &stubDiscovery{}, &stubDiscovery{}
	var drv recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.instances.demo.service-name", "user-svc")
		app.Property("spring.http-client.instances.demo.discovery", "nacos")
		app.Provide(func() discovery.Discovery { return nacos }).Name("nacos")
		app.Provide(func() discovery.Discovery { return consul }).Name("consul")
		app.Provide(func() Driver { return &drv })
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:""`
	}) {
		if ts.DT == nil {
			t.Fatal("expected a wired dispatch transport")
		}
		// The label picks among several backends: the driver must get the one the
		// entry cites, not merely some backend.
		if drv.backend != discovery.Discovery(nacos) {
			t.Fatalf("expected the driver to receive the bean the label cites, got %#v", drv.backend)
		}
	})
}

// TestDriverBeanNamedSelection proves the per-entry
// ${spring.http-client.instances.<name>.driver} key selects a Driver bean by
// NAME: with two Driver beans in the container, the entry cites one and its
// transport is assembled through exactly that one.
func TestDriverBeanNamedSelection(t *testing.T) {
	var first, second recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.instances.demo.addr", "10.0.0.1:8080")
		app.Property("spring.http-client.instances.demo.driver", "corp")
		app.Provide(func() Driver { return &first }).Name("plain")
		app.Provide(func() Driver { return &second }).Name("corp")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:""`
	}) {
		if ts.DT == nil {
			t.Fatal("expected a wired dispatch transport")
		}
		if first.called || !second.called {
			t.Fatalf("expected the transports to go through the named driver only: plain=%v corp=%v", first.called, second.called)
		}
	})
}

// TestDriverSelectionIsPerEntry proves two entries may cite different Driver
// beans: driver selection is per entry, not process-wide.
func TestDriverSelectionIsPerEntry(t *testing.T) {
	var first, second recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.instances.one.addr", "10.0.0.1:8080")
		app.Property("spring.http-client.instances.one.driver", "plain")
		app.Property("spring.http-client.instances.two.addr", "10.0.0.2:8080")
		app.Property("spring.http-client.instances.two.driver", "corp")
		app.Provide(func() Driver { return &first }).Name("plain")
		app.Provide(func() Driver { return &second }).Name("corp")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:""`
	}) {
		if ts.DT == nil {
			t.Fatal("expected a wired dispatch transport")
		}
		if !first.called || !second.called {
			t.Fatalf("expected both named drivers to assemble their own entry: plain=%v corp=%v", first.called, second.called)
		}
	})
}

// TestDuplicateTargetsRejected proves two entries claiming the same target fail
// wiring instead of one silently shadowing the other.
func TestDuplicateTargetsRejected(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.instances.one.addr", "10.0.0.1:8080")
		app.Property("spring.http-client.instances.two.addr", "10.0.0.1:8080")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:"?"`
	}) {
		if ts.DT != nil {
			t.Fatal("expected the duplicate target to fail wiring")
		}
	})
}

// TestBucketNamesAreLegalInstanceNames is the regression for the collision class
// the two-bucket namespace removes: an instance may be named after a bucket
// ("instances", "default") or after a family-wide key ("driver"), because
// instance names live in their own bucket and there are no reserved words.
func TestBucketNamesAreLegalInstanceNames(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.instances.driver.addr", "10.0.0.9:80")
		app.Property("spring.http-client.instances.default.addr", "10.0.0.7:80")
		app.Property("spring.http-client.instances.instances.addr", "10.0.0.6:80")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:""`
	}) {
		if ts.DT == nil {
			t.Fatal("expected a wired dispatch transport")
		}
		for _, addr := range []string{"10.0.0.9:80", "10.0.0.7:80", "10.0.0.6:80"} {
			if _, err := ts.DT.transport(addr); err != nil {
				t.Fatalf("instance %s was not registered: %v", addr, err)
			}
		}
	})
}

// TestFamilyDefaultDriver proves ${spring.http-client.default.driver} is the
// family-wide fallback: an instance that names no driver of its own uses it.
func TestFamilyDefaultDriver(t *testing.T) {
	var corp recordingDriver
	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.default.driver", "corp")
		app.Property("spring.http-client.instances.one.addr", "10.0.0.1:8080")
		app.Property("spring.http-client.instances.two.addr", "10.0.0.2:8080")
		app.Provide(func() Driver { return &corp }).Name("corp")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:""`
	}) {
		if ts.DT == nil {
			t.Fatal("expected a wired dispatch transport")
		}
		if !corp.called {
			t.Fatal("expected the family-wide default driver to assemble the instances")
		}
	})
}

// TestDefaultsAloneDoNotActivate proves the starter stays out of the process when
// only family-wide defaults are set: ${spring.http-client.default} configures no
// client, so it must not take over httpclt.DoRequest with an empty route table.
func TestDefaultsAloneDoNotActivate(t *testing.T) {
	prev := httpclt.DoRequest
	defer func() { httpclt.DoRequest = prev }()

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.http-client.default.driver", "corp")
	}).RunTest(t, func(ts *struct {
		DT *dispatchTransport `autowire:"?"`
	}) {
		if ts.DT != nil {
			t.Fatal("expected no dispatch transport when only defaults are configured")
		}
	})
	if reflect.ValueOf(httpclt.DoRequest).Pointer() != reflect.ValueOf(prev).Pointer() {
		t.Fatal("expected httpclt.DoRequest to be left alone")
	}
}
