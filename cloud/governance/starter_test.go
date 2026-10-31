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
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/capability/traffic"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/spring/gs"
)

// pushSource is source.NewPushSource for a Config built in code.
func pushSource(cfg resilience.LimitsSettings) *source.PushSource[resilience.LimitsSettings] {
	return source.NewPushSource(cfg)
}

// recordingDriver is a backend that flags every executor it builds, so a test
// can prove WHICH driver the runtime selected rather than merely that some
// executor exists.
type recordingDriver struct{ used bool }

func (d *recordingDriver) NewExecutor(service string, m *resilience.Manager) (chain.Executor, error) {
	return recordingExecutor{mark: func() { d.used = true }}, nil
}

// recordingExecutor runs fn and marks its driver used.
type recordingExecutor struct{ mark func() }

func (e recordingExecutor) Execute(ctx context.Context, call chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	e.mark()
	return fn(ctx)
}

func (recordingExecutor) Close() error { return nil }

// managerProbe is a placeholder bean: its only job is to be constructed with
// the resilience manager injected, so a test can capture the very instance the
// runtime also drives. It carries no behaviour.
type managerProbe struct{}

// testCfg builds a limits document with a timeout.
func testCfg(timeoutMs int) resilience.LimitsSettings {
	return resilience.LimitsSettings{Rules: []resilience.LimitsRule{{
		Service: "x",
		Action:  chain.ActionMatch{Exact: resilience.ServiceScopeAction},
		Limits:  resilience.Limits{Timeout: time.Duration(timeoutMs) * time.Millisecond},
	}}}
}

// TestStarter_SourceBeanReachesTheRuntime is the regression guard for the
// source's container route: a bean exported as source.Source[resilience.LimitsSettings] must reach the
// runtime's constructor — there is no install step in between — so its snapshot
// is what the module authorities run on before the app serves. Dropping the
// interface injection here must fail this test.
func TestStarter_SourceBeanReachesTheRuntime(t *testing.T) {
	var resClient *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		// Declared with the interface return type, which is what makes the bean
		// visible to the runtime's Source parameter.
		app.Provide(func() source.Source[resilience.LimitsSettings] {
			return pushSource(testCfg(250))
		})
		// A stand-in for a client starter: it injects a module authority, which
		// is the same instance the runtime drives.
		app.Provide(func(m *resilience.Manager) *managerProbe {
			resClient = m
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if resClient == nil {
			t.Fatal("the resilience manager bean was not injected")
		}
		if p := resClient.LimitsFor("x"); p.Timeout != 250*time.Millisecond {
			t.Fatalf("a bean exported as source.Source[resilience.LimitsSettings] must be the runtime's source: want 250ms, got %v", p.Timeout)
		}
	})
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
		app.Provide(func() source.Source[resilience.LimitsSettings] {
			return pushSource(resilience.LimitsSettings{})
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
		exec := mgr.ExecutorFor("res:1")
		if err := exec.Execute(context.Background(), chain.Operation{}, nil, func(context.Context) error { return nil }); err != nil {
			t.Fatal(err)
		}
		if !drv.used {
			t.Fatal("spring.resilience.driver=corp must build through the corp driver bean")
		}
	})
}

// TestRuntime_IsTheInjectedBean pins that the composition root is what the
// container hands out: a starter that injects *Runtime sees the same authority
// instances the control plane pushes into.
func TestRuntime_IsTheInjectedBean(t *testing.T) {
	var rt *Runtime

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() source.Source[resilience.LimitsSettings] {
			return pushSource(testCfg(120))
		})
		app.Provide(func(r *Runtime) *managerProbe {
			rt = r
			return &managerProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if rt == nil {
			t.Fatal("the runtime bean was not injected")
		}
		if p := rt.ResilienceClient().LimitsFor("x"); p.Timeout != 120*time.Millisecond {
			t.Fatalf("the injected runtime must be the armed one: want 120ms, got %v", p.Timeout)
		}
		if rt.GuardServer() == nil || rt.FaultClient() == nil || rt.FaultServer() == nil {
			t.Fatal("the runtime must expose every authority")
		}
	})
}

// TestRuntime_HandsOutDiscovery pins the one authority that is not a policy
// target: the runtime carries the discovery directory and returns exactly the
// manager it was built with, so a client reaches discovery through the same
// injection point as the policy authorities. The control plane neither holds nor
// dispatches it.
func TestRuntime_HandsOutDiscovery(t *testing.T) {
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "10.0.0.1:2379"})
	disc := discovery.NewManager(map[string]discovery.Discovery{"etcd.main": backend})

	rt := &Runtime{
		Resilience:  resilience.NewManager(resilience.NewDefaultDriver(nil)),
		LoadBalance: loadbalance.NewManager(),
		Admission:   admission.NewManager(),
		ClientFault: clientfault.NewManager(),
		ServerFault: serverfault.NewManager(),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
		Directory:   disc,
	}
	if _, err := loadbalanceplane.NewCenter(context.Background(), rt.LoadBalance, source.NewPushSource(loadbalance.Settings{})); err != nil {
		t.Fatal(err)
	}
	if _, err := admissionplane.NewCenter(context.Background(), rt.Admission, source.NewPushSource(admission.Settings{})); err != nil {
		t.Fatal(err)
	}
	if rt.Discovery() != disc {
		t.Fatal("the runtime must hand out the discovery manager it was built with")
	}
	got, ok := rt.Discovery().Get("etcd.main")
	if !ok || got != backend {
		t.Fatalf("discovery lookup: ok=%v backend=%v", ok, got)
	}
}
