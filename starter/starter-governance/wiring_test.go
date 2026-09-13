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

package StarterGovernance

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// recordingDriver is a backend that flags every executor it builds, so a test
// can prove WHICH driver the center selected rather than merely that some
// executor exists.
type recordingDriver struct{ used bool }

func (d *recordingDriver) NewExecutor(resilience.Policy) (resilience.Executor, error) {
	return recordingExecutor{mark: func() { d.used = true }}, nil
}

// recordingExecutor runs fn and marks its driver used.
type recordingExecutor struct{ mark func() }

func (e recordingExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingExecutor) Refresh(resilience.Policy) error { return nil }

func (recordingExecutor) Close() error { return nil }

// govCfg builds a governance.Config with the given default attempt timeout and
// an armed fault section.
func govCfg(timeoutMs int, fc fault.Config) governance.Config {
	return governance.Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: time.Duration(timeoutMs) * time.Millisecond},
		Fault:   fc,
	}
}

// TestWiring_SrcBeanArmsCenter drives the wiring bean's normal branch: a Source
// bean is present, so Init binds it and GoLive arms the center from its snapshot
// — the same sequence gs runs on the wiring bean in production.
//
// It cannot go through gs.RunTest: gs_init.Beans() CLONES global bean
// definitions under testing.Testing() (test isolation), so a test binary wires
// a copy of the bean while the governance facade still reads the package
// singleton. Driving Init directly covers the same code path without that gap.
func TestWiring_SrcBeanArmsCenter(t *testing.T) {
	defer governance.Reset()

	w := newWiring()
	w.Src = governance.NewPushSource(govCfg(300, fault.Config{Enabled: true, Rate: 0.25}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	if !governance.Enabled() {
		t.Fatal("wiring should arm the center from the Source bean's snapshot")
	}
	if p := governance.PolicyFor("redis:cache"); p.Timeout != 300*time.Millisecond {
		t.Fatalf("src bean policy: want 300ms, got %v", p.Timeout)
	}
	if in := fault.InjectorFor(); in == nil || !in.Config().Enabled || in.Config().Rate != 0.25 {
		t.Fatal("GoLive should register the injector built from the snapshot's Fault")
	}
	ready := false
	governance.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the authority live")
	}
}

// TestWiring_NoSource_StaysDisabled pins the no-source contract: with no Source
// bean Init still runs GoLive (registering the seams and firing OnReady) but the
// center stays disabled, so every client resolves a transparent pass-through.
func TestWiring_NoSource_StaysDisabled(t *testing.T) {
	defer governance.Reset()

	w := newWiring()
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	if governance.Enabled() {
		t.Fatal("no source configured: center must stay disabled")
	}
	if p := governance.PolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("disabled center must resolve a zero policy, got %v", p)
	}
	ready := false
	governance.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the authority live even with no source")
	}
}

// TestWiring_ExplicitSetSourceWinsOverSrc covers the top of the priority chain:
// a SetSource called before wiring pre-empts the Src bean (BindDefault is a
// no-op when a source is already bound).
func TestWiring_ExplicitSetSourceWinsOverSrc(t *testing.T) {
	defer governance.Reset()

	governance.SetSource(governance.NewPushSource(govCfg(200, fault.Config{})))

	w := newWiring()
	w.Src = governance.NewPushSource(govCfg(300, fault.Config{}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}
	if p := governance.PolicyFor("x"); p.Timeout != 200*time.Millisecond {
		t.Fatalf("explicit SetSource should win: want 200ms, got %v", p.Timeout)
	}
}

// TestWiring_DriverDirectorySelectsBackend pins the driver contract: the
// directory the wiring bean injects is what govern.driver resolves against, so
// naming a backend builds through THAT backend — not through the bundled one.
func TestWiring_DriverDirectorySelectsBackend(t *testing.T) {
	defer governance.Reset()

	drv := &recordingDriver{}
	w := newWiring()
	w.Src = governance.NewPushSource(governance.Config{Enabled: true, Driver: "custom"})
	w.Drivers = map[string]resilience.Driver{"custom": drv}
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	exec, err := governance.NewExecutor(resilience.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Execute(context.Background(), "res:1", func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !drv.used {
		t.Fatal("govern.driver=custom must build through the directory's custom driver")
	}
}

// TestWiring_UnknownDriverFailsStartup pins the fail-loud contract: an enabled
// center whose configured driver matches nothing aborts wiring rather than
// silently serving a pass-through, which would look like "resilience is on".
func TestWiring_UnknownDriverFailsStartup(t *testing.T) {
	defer governance.Reset()

	w := newWiring()
	w.Src = governance.NewPushSource(governance.Config{Enabled: true, Driver: "nope"})
	w.Drivers = map[string]resilience.Driver{"custom": &recordingDriver{}}
	assert.Panic(t, func() { _ = w.Init() },
		`resilience: no driver named "nope" \(available: \[custom default\]\)`)
}

// TestWiring_DriverBeansAreCollectedFromContainer is the regression guard for
// the whole refactor: a backend contributed as a named bean — the way
// starter-governance-sentinel and starter-luohua contribute theirs — must be collected
// by the wiring bean's directory field, rooted, and selectable by govern.driver.
// The Export is load-bearing (gs indexes beans by exact type), so dropping it
// here must fail this test.
func TestWiring_DriverBeansAreCollectedFromContainer(t *testing.T) {
	defer governance.Reset()

	drv := &recordingDriver{}
	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() governance.Source {
			return governance.NewPushSource(governance.Config{Enabled: true, Driver: "corp"})
		})
		app.Provide(func() *recordingDriver { return drv }).
			Name("corp").
			Export(gs.As[resilience.Driver]())
	}).RunTest(t, func(_ *struct{}) {
		exec := resilience.ExecutorFor("sys", "res:1")
		if err := exec.Execute(context.Background(), "res:1", func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if !drv.used {
			t.Fatal("a driver bean exported as resilience.Driver must reach the center's directory")
		}
	})
}

// TestWiring_DriverBeanWithoutExportFailsStartup pins the other half of the
// directory contract: gs indexes beans by their exact type, so a backend that
// forgets Export(gs.As[resilience.Driver]()) is invisible to the directory —
// and naming it must fail startup rather than silently falling back to the
// bundled driver. This is the most likely mistake a new backend module makes.
func TestWiring_DriverBeanWithoutExportFailsStartup(t *testing.T) {
	defer governance.Reset()

	assert.Panic(t, func() {
		gs.Web(false).Configure(func(app gs.App) {
			app.Provide(func() governance.Source {
				return governance.NewPushSource(governance.Config{Enabled: true, Driver: "corp"})
			})
			// No Export: the concrete *recordingDriver is indexed under its own type.
			app.Provide(func() *recordingDriver { return &recordingDriver{} }).Name("corp")
		}).RunTest(t, func(_ *struct{}) {})
	}, `no driver named "corp"`)
}
