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

func (e recordingServerExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingServerExecutor) Close() error { return nil }

// recordingExecutor runs fn and marks its driver used.
type recordingExecutor struct{ mark func() }

func (e recordingExecutor) Execute(ctx context.Context, call chain.Operation, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingExecutor) Close() error { return nil }

// govCfg builds a document with an outbound timeout and a CLIENT-side fault
// fire; the server side is covered by the same shape on the other block.
func govCfg(timeoutMs int, fc fault.Config) Config {
	return Config{
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
	return newTestStartupWith(NewPushSource(Config{}))
}

// newTestStartupWith is newTestStartup over the given source — the test's
// stand-in for what the container hands the center's constructor in production.
// The resilience authority runs on the bundled driver: the backend is a startup
// choice made at wiring, not part of the document a center test drives.
func newTestStartupWith(src Source) (*Center, *resilience.Manager, *loadbalance.Manager, *fault.Injector) {
	lb := loadbalance.NewManager()
	res := resilience.NewManager(nil)
	inj := fault.NewInjector(fault.Configs{Client: fault.Config{}}, nil)
	c, err := NewCenter(res, lb, inj, nil, src)
	if err != nil {
		panic(err)
	}
	return c, res, lb, inj
}

// TestStarter_SrcBeanArmsCenter drives the normal branch: a Source is present, so
// the center binds it at construction and GoLive distributes its snapshot into
// the module authorities — the same sequence gs runs in production.
func TestStarter_SrcBeanArmsCenter(t *testing.T) {
	_, res, _, inj := newTestStartupWith(
		NewPushSource(govCfg(300, fault.Config{Rate: 0.25})))

	if p := res.ClientPolicyFor("redis:cache"); p.AttemptTimeout != 300*time.Millisecond {
		t.Fatalf("src bean policy: want 300ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); cf.Rate != 0.25 {
		t.Fatal("GoLive should push the snapshot's Fault into the injected injector")
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

// TestStarter_EmptySource_StaysUnarmed pins the empty-document contract: a
// source that holds the zero Config still arms the center (Init runs GoLive and
// the center goes live) but every service resolves a transparent pass-through.
// The absence of a Source bean is a different failure — the container fails
// startup on the center's required Source parameter, so there is no nil-source
// path to pin here.
func TestStarter_EmptySource_StaysUnarmed(t *testing.T) {
	_, res, lb, inj := newTestStartup()

	if p := res.ClientPolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("empty source: resilience must resolve a zero policy, got %v", p)
	}
	// The manager exposes no read-back — production only ever pushes into sinks —
	// so ask through one: a chain registers with the selection already in force.
	lbChain := lb.Chain(context.Background(), func() ([]discovery.Endpoint, error) { return nil, nil }, "redis:cache")
	if got := lbChain.Selection(); got.Balancer != "" || got.OutlierThreshold != 0 {
		t.Fatalf("empty source: selection must resolve zero, got %+v", got)
	}
	if !inj.ClientConfig().Inert() || !inj.ServerConfig().Inert() {
		t.Fatal("empty source: fault must stay inert")
	}
}

// TestStarter_LatePushDrivesArmedModules pins the push path end to end: after
// Init, a source push reaches the module authorities with no further setup step.
func TestStarter_LatePushDrivesArmedModules(t *testing.T) {
	src := NewPushSource(govCfg(100, fault.Config{}))
	_, res, _, inj := newTestStartupWith(src)

	// A live push swaps both the resilience side and the fault side.
	src.Push(context.Background(), govCfg(400, fault.Config{Rate: 0.5}))
	if p := res.ClientPolicyFor("x"); p.AttemptTimeout != 400*time.Millisecond {
		t.Fatalf("push should reach the resilience manager: want 400ms, got %v", p.AttemptTimeout)
	}
	if cf := inj.ClientConfig(); cf.Rate != 0.5 {
		t.Fatalf("push should reach the injector: %+v", cf)
	}
}

// TestStarter_DriverBeanSelectedByName is the regression guard for the driver
// wiring: ${spring.resilience.driver} names the [resilience.Driver] bean the
// manager is built over — the way the sentinel and luohua backends contribute
// theirs — so naming a backend builds through THAT backend. The Export is
// load-bearing (gs indexes beans by exact type), so dropping it would leave the
// named lookup with nothing to inject and fail startup.
func TestStarter_DriverBeanSelectedByName(t *testing.T) {
	drv := &recordingDriver{}
	var mgr *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		app.Property(resilience.DriverProperty, "corp")
		app.Provide(func() Source {
			return NewPushSource(Config{})
		})
		app.Provide(func() *recordingDriver { return drv }).
			Name("corp").
			Export(gs.As[resilience.Driver]())
		// A stand-in for a client starter: it injects the manager bean, so gs
		// instantiates it and this test can capture the instance the container
		// built with the selected driver.
		app.Provide(func(m *resilience.Manager) *managerProbe {
			mgr = m
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if mgr == nil {
			t.Fatal("the resilience manager bean was not injected")
		}
		exec := mgr.ClientExecutorFor("sys", "res:1")
		if err := exec.Execute(context.Background(), chain.Operation{}, func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if !drv.used {
			t.Fatal("spring.resilience.driver=corp must build through the corp driver bean")
		}
	})
}

// TestStarter_UnknownBalancerFailsStartup pins the fail-loud contract for the
// selection half: a document naming a strategy this process never registered
// aborts startup, instead of silently serving chains on the base strategy while
// the operator believes their rule took effect.
func TestStarter_UnknownBalancerFailsStartup(t *testing.T) {
	_, err := NewCenter(resilience.NewManager(nil), loadbalance.NewManager(),
		fault.NewInjector(fault.Configs{}, nil), nil,
		NewPushSource(selectionConfig("redis:cache", "no_such_strategy", 0, 0)))
	if err == nil {
		t.Fatal("an unregistered strategy name must fail construction")
	}
	if !strings.Contains(err.Error(), "no strategy registered") {
		t.Fatalf("error should name the missing strategy, got: %v", err)
	}
}
