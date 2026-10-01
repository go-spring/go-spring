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
	"strings"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/spring/gs"
)

// recordingDriver is a backend that flags every executor it builds, so a test
// can prove WHICH driver the center selected rather than merely that some
// executor exists.
type recordingDriver struct{ used bool }

func (d *recordingDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (resilience.ClientExecutor, error) {
	return recordingExecutor{mark: func() { d.used = true }}, nil
}

func (d *recordingDriver) NewServerExecutor(service string, a resilience.ServerPolicy) (resilience.ServerExecutor, error) {
	return recordingServerExecutor{mark: func() { d.used = true }}, nil
}

// recordingServerExecutor is the inbound twin of recordingExecutor.
type recordingServerExecutor struct{ mark func() }

func (e recordingServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingServerExecutor) Refresh(resilience.ServerPolicy) error { return nil }

func (recordingServerExecutor) Close() error { return nil }

// recordingExecutor runs fn and marks its driver used.
type recordingExecutor struct{ mark func() }

func (e recordingExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingExecutor) Refresh(resilience.ClientPolicy) error { return nil }

func (recordingExecutor) Close() error { return nil }

// govCfg builds a document with an outbound timeout and a CLIENT-side fault
// fire; the server side is covered by the same shape on the other block.
func govCfg(timeoutMs int, fc fault.Config) governance.Config {
	return governance.Config{
		Enabled: true,
		Client: governance.ClientConfig{
			Default: governance.ClientDefaultPolicy{
				ClientPolicy: resilience.ClientPolicy{AttemptTimeout: time.Duration(timeoutMs) * time.Millisecond},
			},
			Fault: fc,
		},
	}
}

// managerProbe is a placeholder bean: its only job is to be constructed with
// the resilience manager injected, so a test can capture the very instance the
// wiring also drives. It carries no behaviour.
type managerProbe struct{}

// newTestWiring builds the wiring over a fresh set of authorities, exactly as
// this package's init registers them as beans: one instance per module, shared
// between the center and the callers that inject them.
func newTestWiring() (*wiring, *resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	res := resilience.NewManager()
	lb := loadbalance.NewManager()
	inj := fault.NewInjector(fault.Configs{Client: fault.Config{}}, nil)
	return &wiring{Ctr: governance.NewCenter(governance.Config{}, res, lb, inj)}, res, lb, inj
}

// TestWiring_SrcBeanArmsCenter drives the wiring bean's normal branch: a Source
// bean is present, so Init binds it and GoLive distributes its snapshot into the
// module authorities — the same sequence gs runs on the wiring bean in production.
func TestWiring_SrcBeanArmsCenter(t *testing.T) {
	w, res, _, inj := newTestWiring()

	w.Src = governance.NewPushSource(govCfg(300, fault.Config{Enabled: true, Rate: 0.25}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	if p := res.ClientPolicyFor("redis:cache"); p.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("src bean policy: want 300ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.25 {
		t.Fatal("GoLive should push the snapshot's Fault into the injected injector")
	}
	ready := false
	w.Ctr.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the center live")
	}
	if !w.Ctr.Live() {
		t.Fatal("Live() should report the center armed")
	}
}

// TestWiring_NoSource_StaysDisabled pins the no-source contract: with no Source
// bean Init still runs GoLive (distributing an empty document and firing
// OnReady) but every service resolves a transparent pass-through.
func TestWiring_NoSource_StaysDisabled(t *testing.T) {
	w, res, lb, inj := newTestWiring()

	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	if p := res.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("no source: resilience must resolve a zero policy, got %v", p)
	}
	if lb.Enabled() {
		t.Fatal("no source: selection must stay unarmed")
	}
	if inj.ClientConfig().Enabled || inj.ServerConfig().Enabled {
		t.Fatal("no source: fault must stay disabled")
	}
	ready := false
	w.Ctr.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the center live even with no source")
	}
}

// TestWiring_ExplicitSetSourceWinsOverSrc covers the top of the priority chain:
// a SetSource called before wiring pre-empts the Src bean (BindDefault is a
// no-op when a source is already bound).
func TestWiring_ExplicitSetSourceWinsOverSrc(t *testing.T) {
	w, res, _, _ := newTestWiring()

	w.Ctr.SetSource(governance.NewPushSource(govCfg(200, fault.Config{})))
	w.Src = governance.NewPushSource(govCfg(300, fault.Config{}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}
	if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 200*time.Millisecond {
		t.Fatalf("explicit SetSource should win: want 200ms, got %v", p.AttemptTimeout)
	}
}

// TestWiring_LatePushDrivesArmedModules pins the push path end to end: after
// Init, a source push reaches the module authorities without any further wiring.
func TestWiring_LatePushDrivesArmedModules(t *testing.T) {
	w, res, _, inj := newTestWiring()

	src := governance.NewPushSource(govCfg(100, fault.Config{}))
	w.Src = src
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	// A live push swaps both the resilience side and the fault side.
	src.Push(govCfg(400, fault.Config{Enabled: true, Rate: 0.5}))
	if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 400*time.Millisecond {
		t.Fatalf("push should reach the resilience manager: want 400ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.5 {
		t.Fatalf("push should reach the injector: %+v", cf)
	}
}

// TestWiring_DriverDirectorySelectsBackend pins the driver contract: the
// directory the wiring bean injects is what spring.governance.driver resolves against, so
// naming a backend builds through THAT backend — not through the bundled one.
func TestWiring_DriverDirectorySelectsBackend(t *testing.T) {
	w, res, _, _ := newTestWiring()

	drv := &recordingDriver{}
	w.Src = governance.NewPushSource(governance.Config{Enabled: true, Driver: "custom"})
	w.Drivers = map[string]resilience.Driver{"custom": drv}
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	exec, err := res.NewClientExecutor("svc", resilience.ClientPolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !drv.used {
		t.Fatal("spring.governance.driver=custom must build through the directory's custom driver")
	}
	if d := res.Driver(); d != "custom" {
		t.Fatalf("Driver() = %q, want custom", d)
	}
}

// TestWiring_UnknownDriverFailsStartup pins the fail-loud contract: an enabled
// center whose configured driver matches nothing aborts wiring rather than
// silently serving a pass-through, which would look like "resilience is on".
func TestWiring_UnknownDriverFailsStartup(t *testing.T) {
	w, _, _, _ := newTestWiring()

	w.Src = governance.NewPushSource(governance.Config{Enabled: true, Driver: "nope"})
	w.Drivers = map[string]resilience.Driver{"custom": &recordingDriver{}}
	err := w.Init()
	if err == nil {
		t.Fatal("an uninstalled driver name must fail Init")
	}
	if !strings.Contains(err.Error(), `no driver named "nope"`) {
		t.Fatalf("error should name the missing driver, got: %v", err)
	}
}

// TestWiring_DriverBeansAreCollectedFromContainer is the regression guard for
// the directory's container route: a backend contributed as a named bean — the
// way the sentinel and luohua backends contribute theirs — must reach the
// manager the wiring arms, and a client that injects that manager must build
// through it. The Export is load-bearing (gs indexes beans by exact type), so
// dropping it here must fail this test.
func TestWiring_DriverBeansAreCollectedFromContainer(t *testing.T) {
	drv := &recordingDriver{}
	var mgr *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() governance.Source {
			return governance.NewPushSource(governance.Config{Enabled: true, Driver: "corp"})
		})
		app.Provide(func() *recordingDriver { return drv }).
			Name("corp").
			Export(gs.As[resilience.Driver]())
		// A stand-in for a client starter: it injects the manager bean the
		// wiring also drives, which is what makes the directory shared. It is
		// exported as a Rooter so gs instantiates it despite nothing autowiring
		// it; capturing at construction is enough, since the assertion only
		// needs the instance.
		app.Provide(func(m *resilience.Manager) *managerProbe {
			mgr = m
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if mgr == nil {
			t.Fatal("the resilience manager bean was not injected")
		}
		exec := mgr.ClientExecutorFor("sys", "res:1")
		if err := exec.Execute(context.Background(), func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if !drv.used {
			t.Fatal("a driver bean exported as resilience.Driver must reach the center's directory")
		}
	})
}

// TestWiring_DriverBeanWithoutExportIsInvisible pins the other half of the
// directory contract: gs indexes beans by their exact type, so a backend that
// forgets Export(gs.As[resilience.Driver]()) never reaches the directory — and a
// config naming it then fails startup rather than silently falling back to the
// bundled driver. This is the most likely mistake a new backend module makes.
//
// The source here is left disabled so the app still starts and the collected
// directory can be inspected directly; the startup failure a real config would
// trigger is pinned by TestWiring_UnknownDriverFailsStartup.
func TestWiring_DriverBeanWithoutExportIsInvisible(t *testing.T) {
	var captured *wiring

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() governance.Source {
			return governance.NewPushSource(governance.Config{})
		})
		// No Export: the concrete *recordingDriver is indexed under its own type.
		app.Provide(func() *recordingDriver { return &recordingDriver{} }).Name("corp")
		app.Provide(func(w *wiring) *wiringProbe { captured = w; return &wiringProbe{} }).
			Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if captured == nil {
			t.Fatal("the wiring bean was not constructed")
		}
		if _, ok := captured.Drivers["corp"]; ok {
			t.Fatal("a driver bean without Export must be invisible to the directory")
		}
	})
}

// TestWiring_BalancerFactoryBeansAreCollectedFromContainer is the load-balancing
// counterpart of the driver-directory guard: a strategy contributed as a named
// bean — the way a company contributes its own balancer — must reach the manager
// the wiring arms, and a pool bound to a rule naming it must actually build
// through it. That is the whole "new strategy, with parameters of its own,
// without touching cloud/loadbalance" promise.
func TestWiring_BalancerFactoryBeansAreCollectedFromContainer(t *testing.T) {
	var captured *wiring
	fac := &recordingFactory{}
	var pool *loadbalance.Pool
	var mgr *loadbalance.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() governance.Source {
			return governance.NewPushSource(governance.Config{
				Enabled: true,
				Client: governance.ClientConfig{
					Rules: []governance.ClientRule{{
						Service: "sys",
						Selection: loadbalance.Selection{
							Balancer: "corp",
							Params:   map[string]string{"size": "7"},
						},
					}},
				},
			})
		})
		app.Provide(func() loadbalance.Factory { return fac }).
			Name("corp").
			Export(gs.As[loadbalance.Factory]())
		app.Provide(func(w *wiring) *wiringProbe { captured = w; return &wiringProbe{} }).
			Export(gs.As[gs.Rooter]())
		// A stand-in for a client starter: it injects the manager bean the wiring
		// also drives, then binds a pool to the governed label.
		app.Provide(func(m *loadbalance.Manager) *managerProbe {
			mgr = m
			pool = loadbalance.NewPool(func() ([]discovery.Endpoint, error) {
				return []discovery.Endpoint{{Addr: "10.0.0.1:80", Healthy: true, Weight: 1}}, nil
			}, loadbalance.NewRoundRobin())
			m.Bind(pool, "sys")
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if captured == nil || mgr == nil || pool == nil {
			t.Fatal("the wiring or client beans were not constructed")
		}
		if _, ok := captured.Factories["corp"]; !ok {
			t.Fatal("a factory bean exported as loadbalance.Factory must reach the wiring's directory")
		}
		if _, err := pool.Pick(loadbalance.PickInfo{}); err != nil {
			t.Fatal(err)
		}
		if !fac.used || fac.size != 7 {
			t.Fatalf("the pool must have been built through the contributed factory with its params, used=%v size=%d", fac.used, fac.size)
		}
		if pool.Selection().Balancer != "corp" {
			t.Fatalf("the pool's strategy: want corp, got %q", pool.Selection().Balancer)
		}
	})
}

// recordingFactory is a contributed strategy that flags every build, so a test
// can prove the pool resolved through it — and can see the parameter it read.
type recordingFactory struct {
	used bool
	size int
}

func (f *recordingFactory) Build(_ loadbalance.Directory, p *loadbalance.Params) (loadbalance.Balancer, error) {
	size, err := p.Int("size", 1)
	if err != nil {
		return nil, err
	}
	if err := p.Done(); err != nil {
		return nil, err
	}
	f.used, f.size = true, size
	return loadbalance.NewRoundRobin(), nil
}

// wiringProbe is a placeholder bean whose only job is to be constructed with
// the wiring bean injected, so a test can inspect what the container handed it.
type wiringProbe struct{}
