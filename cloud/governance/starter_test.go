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

package governance

import (
	"context"
	"go-spring.org/cloud/chain"
	"strings"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
)

// recordingDriver is a backend that flags every executor it builds, so a test
// can prove WHICH driver the center selected rather than merely that some
// executor exists.
type recordingDriver struct{ used bool }

func (d *recordingDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (chain.Executor, error) {
	return recordingExecutor{mark: func() { d.used = true }}, nil
}

func (d *recordingDriver) NewServerExecutor(service string, a resilience.ServerPolicy) (chain.Executor, error) {
	return recordingServerExecutor{mark: func() { d.used = true }}, nil
}

// recordingServerExecutor is the inbound twin of recordingExecutor.
type recordingServerExecutor struct{ mark func() }

func (e recordingServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingServerExecutor) Close() error { return nil }

// recordingExecutor runs fn and marks its driver used.
type recordingExecutor struct{ mark func() }

func (e recordingExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingExecutor) Close() error { return nil }

// govCfg builds a document with an outbound timeout and a CLIENT-side fault
// fire; the server side is covered by the same shape on the other block.
func govCfg(timeoutMs int, fc fault.Config) Config {
	return Config{
		Enabled: true,
		Client: ClientConfig{
			Default: ClientDefaultPolicy{
				ClientPolicy: resilience.ClientPolicy{AttemptTimeout: time.Duration(timeoutMs) * time.Millisecond},
			},
			Fault: fc,
		},
	}
}

// managerProbe is a placeholder bean: its only job is to be constructed with
// the resilience manager injected, so a test can capture the very instance the
// center also drives. It carries no behaviour.
type managerProbe struct{}

// newTestCenter builds the center bean over a fresh set of authorities, exactly
// as this package's init registers them: one instance per module, shared between
// the center and the callers that inject them. GoLive is NOT called — a caller
// that needs the center live calls it itself, the way the container does through
// the bean's Init hook.
func newTestStartup() (*Center, *resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	return newTestStartupWith(nil, nil, nil)
}

// newTestCenterWith is newTestCenter over authorities carrying the given
// contributed backends and source — the test's stand-in for what the container
// hands each bean's constructor in production.
func newTestStartupWith(drivers map[string]resilience.Driver, factories map[string]loadbalance.Factory,
	src Source) (*Center, *resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	lb, err := loadbalance.NewManager(factories)
	if err != nil {
		panic(err)
	}
	res := resilience.NewManager(drivers)
	inj := fault.NewInjector(fault.Configs{Client: fault.Config{}}, nil)
	return NewCenter(Config{}, res, lb, inj, nil, src), res, lb, inj
}

// TestStarter_SrcBeanArmsCenter drives the normal branch: a Source is present, so
// the center binds it at construction and GoLive distributes its snapshot into
// the module authorities — the same sequence gs runs in production.
func TestStarter_SrcBeanArmsCenter(t *testing.T) {
	c, res, _, inj := newTestStartupWith(nil, nil,
		NewPushSource(govCfg(300, fault.Config{Enabled: true, Rate: 0.25})))

	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}

	if p := res.ClientPolicyFor("redis:cache"); p.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("src bean policy: want 300ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.25 {
		t.Fatal("GoLive should push the snapshot's Fault into the injected injector")
	}
	ready := false
	c.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the center live")
	}
	if !c.Live() {
		t.Fatal("Live() should report the center armed")
	}
}

// TestStarter_SourceBeanReachesTheCenter is the regression guard for the
// source's container route: a bean exported as governance.Source must reach the
// center through its CONSTRUCTOR — there is no install step in between any more
// — so its snapshot is what the module authorities run on before GoLive even
// fires. Dropping the interface injection here must fail this test.
func TestStarter_SourceBeanReachesTheCenter(t *testing.T) {
	var res *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		// Declared with the interface return type, which is what makes the bean
		// visible to the center's Source parameter.
		app.Provide(func() Source {
			return NewPushSource(govCfg(250, fault.Config{}))
		})
		// A stand-in for a client starter: it injects a module authority, which
		// is the same instance the center drives.
		app.Provide(func(m *resilience.Manager) *managerProbe {
			res = m
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if res == nil {
			t.Fatal("the resilience manager bean was not injected")
		}
		if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 250*time.Millisecond {
			t.Fatalf("a bean exported as governance.Source must be the center's source: want 250ms, got %v", p.AttemptTimeout)
		}
	})
}

// TestStarter_NoSource_StaysDisabled pins the no-source contract: with no Source
// bean Init still runs GoLive (distributing an empty document and firing
// OnReady) but every service resolves a transparent pass-through.
func TestStarter_NoSource_StaysDisabled(t *testing.T) {
	c, res, lb, inj := newTestStartup()

	if err := c.GoLive(); err != nil {
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
	c.OnReady(func() { ready = true })
	if !ready {
		t.Fatal("GoLive should mark the center live even with no source")
	}
}

// TestStarter_SetSourceReplacesTheConstructorSource covers source precedence: a
// SetSource after construction replaces the source the center was built with —
// last write wins, and the replaced source's callbacks go stale via the handle
// guard rather than being retracted.
func TestStarter_SetSourceReplacesTheConstructorSource(t *testing.T) {
	c, res, _, _ := newTestStartupWith(nil, nil, NewPushSource(govCfg(300, fault.Config{})))

	c.SetSource(NewPushSource(govCfg(200, fault.Config{})))
	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}
	if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 200*time.Millisecond {
		t.Fatalf("SetSource should replace the constructor's source: want 200ms, got %v", p.AttemptTimeout)
	}
}

// TestStarter_LatePushDrivesArmedModules pins the push path end to end: after
// Init, a source push reaches the module authorities with no further setup step.
func TestStarter_LatePushDrivesArmedModules(t *testing.T) {
	src := NewPushSource(govCfg(100, fault.Config{}))
	c, res, _, inj := newTestStartupWith(nil, nil, src)

	if err := c.GoLive(); err != nil {
		t.Fatal(err)
	}

	// A live push swaps both the resilience side and the fault side.
	src.Push(context.Background(), govCfg(400, fault.Config{Enabled: true, Rate: 0.5}))
	if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 400*time.Millisecond {
		t.Fatalf("push should reach the resilience manager: want 400ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); !cf.Enabled || cf.Rate != 0.5 {
		t.Fatalf("push should reach the injector: %+v", cf)
	}
}

// TestStarter_DriverDirectorySelectsBackend pins the driver contract: the
// directory the manager was built over is what spring.governance.driver resolves
// against, so naming a backend builds through THAT backend — not through the
// bundled one.
func TestStarter_DriverDirectorySelectsBackend(t *testing.T) {
	drv := &recordingDriver{}
	c, res, _, _ := newTestStartupWith(map[string]resilience.Driver{"custom": drv}, nil,
		NewPushSource(Config{Enabled: true, Driver: "custom"}))

	if err := c.GoLive(); err != nil {
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

// TestStarter_UnknownDriverFailsStartup pins the fail-loud contract: an enabled
// center whose configured driver matches nothing aborts startup rather than
// silently serving a pass-through, which would look like "resilience is on".
func TestStarter_UnknownDriverFailsStartup(t *testing.T) {
	c, _, _, _ := newTestStartupWith(map[string]resilience.Driver{"custom": &recordingDriver{}}, nil,
		NewPushSource(Config{Enabled: true, Driver: "nope"}))

	err := c.GoLive()
	if err == nil {
		t.Fatal("an uninstalled driver name must fail Init")
	}
	if !strings.Contains(err.Error(), `no driver named "nope"`) {
		t.Fatalf("error should name the missing driver, got: %v", err)
	}
}

// TestStarter_DriverBeansAreCollectedFromContainer is the regression guard for
// the directory's container route: a backend contributed as a named bean — the
// way the sentinel and luohua backends contribute theirs — must reach the
// manager the center is built over, and a client that injects that manager must
// through it. The Export is load-bearing (gs indexes beans by exact type), so
// dropping it here must fail this test.
func TestStarter_DriverBeansAreCollectedFromContainer(t *testing.T) {
	drv := &recordingDriver{}
	var mgr *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() Source {
			return NewPushSource(Config{Enabled: true, Driver: "corp"})
		})
		app.Provide(func() *recordingDriver { return drv }).
			Name("corp").
			Export(gs.As[resilience.Driver]())
		// A stand-in for a client starter: it injects the manager bean the
		// center also drives, which is what makes the directory shared. It is
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

// TestStarter_DriverBeanWithoutExportIsInvisible pins the other half of the
// directory contract: gs indexes beans by their exact type, so a backend that
// forgets Export(gs.As[resilience.Driver]()) never reaches the directory — and a
// config naming it then fails startup rather than silently falling back to the
// bundled driver. This is the most likely mistake a new backend module makes.
//
// The source here is left disabled so the app still starts and the manager can
// be exercised directly; the startup failure a real config would trigger is
// pinned by TestStarter_UnknownDriverFailsStartup.
func TestStarter_DriverBeanWithoutExportIsInvisible(t *testing.T) {
	var mgr *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() Source {
			return NewPushSource(Config{})
		})
		// No Export: the concrete *recordingDriver is indexed under its own type.
		app.Provide(func() *recordingDriver { return &recordingDriver{} }).Name("corp")
		app.Provide(func(m *resilience.Manager) *managerProbe {
			mgr = m
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if mgr == nil {
			t.Fatal("the resilience manager bean was not injected")
		}
		// Naming it is a startup error, exactly as if the bean did not exist:
		// the manager was built over a directory that never held it.
		err := mgr.Apply(resilience.Settings{
			Enabled:             true,
			Driver:              "corp",
			ResolveClientPolicy: func(string) resilience.ClientPolicy { return resilience.ClientPolicy{} },
		})
		if err == nil {
			t.Fatal("a driver bean without Export must be invisible to the directory")
		}
	})
}

// TestStarter_BalancerFactoryBeansAreCollectedFromContainer is the load-balancing
// counterpart of the driver-directory guard: a strategy contributed as a named
// bean — the way a company contributes its own balancer — must reach the manager
// the center is built over, and a pool bound to a rule naming it must actually
// through it. That is the whole "new strategy, with parameters of its own,
// without touching cloud/loadbalance" promise.
func TestStarter_BalancerFactoryBeansAreCollectedFromContainer(t *testing.T) {
	fac := &recordingFactory{}
	var pool *loadbalance.Pool
	var mgr *loadbalance.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() Source {
			return NewPushSource(Config{
				Enabled: true,
				Client: ClientConfig{
					Rules: []ClientRule{{
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
		// A stand-in for a client starter: it injects the manager bean the
		// center also drives, then binds a pool to the governed label.
		app.Provide(func(m *loadbalance.Manager) *managerProbe {
			mgr = m
			pool = loadbalance.NewPool(func() ([]discovery.Endpoint, error) {
				return []discovery.Endpoint{{Addr: "10.0.0.1:80", Healthy: true, Weight: 1}}, nil
			}, loadbalance.NewRoundRobin())
			m.Bind(pool, "sys")
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if mgr == nil || pool == nil {
			t.Fatal("the manager or client beans were not constructed")
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
