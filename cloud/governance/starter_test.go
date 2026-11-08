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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	_ "go-spring.org/cloud/governance/controlplane/timeoutretry"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/traffic"
	"go-spring.org/spring/gs"
)

// pushSource is source.NewPushSource for a Config built in code.
func pushSource(cfg resilience.TimeoutRetrySettings) *source.PushSource[resilience.TimeoutRetrySettings] {
	return source.NewPushSource(cfg)
}

// timeoutOf runs a no-op through a fresh executor for label and returns the
// timeout the call resolved under — the public way to observe a pushed policy.
func timeoutOf(t *testing.T, m *resilience.Manager, label string, op chain.Operation) time.Duration {
	t.Helper()
	rec := new(chain.Recorder)
	if err := m.Wrap(label, nil).Execute(context.Background(), op, rec,
		func(context.Context) error { return nil }); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return rec.Timeout
}

// managerProbe is a placeholder bean: its only job is to be constructed with
// the resilience manager injected, so a test can capture the very instance the
// runtime also drives. It carries no behaviour.
type managerProbe struct{}

// testCfg builds a limits document with a timeout.
func testCfg(timeoutMs int) resilience.TimeoutRetrySettings {
	return resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
		Service: "x",
		Rules: []resilience.TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: resilience.TimeoutRetry{Timeout: time.Duration(timeoutMs) * time.Millisecond},
		}},
	}}}
}

// TestStarter_SourceBeanReachesTheRuntime is the regression guard for the
// source's container route: a bean exported as source.Source[resilience.TimeoutRetrySettings] must reach the
// runtime's constructor — there is no install step in between — so its snapshot
// is what the module authorities run on before the app serves. Dropping the
// interface injection here must fail this test.
func TestStarter_SourceBeanReachesTheRuntime(t *testing.T) {
	var resClient *resilience.Manager

	gs.Web(false).Configure(func(app gs.App) {
		// Declared with the interface return type, which is what makes the bean
		// visible to the runtime's Source parameter.
		app.Provide(func() source.Source[resilience.TimeoutRetrySettings] {
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
		if got := timeoutOf(t, resClient, "x", chain.Operation{Service: "x"}); got != 250*time.Millisecond {
			t.Fatalf("a bean exported as source.Source[resilience.TimeoutRetrySettings] must be the runtime's source: want 250ms, got %v", got)
		}
	})
}

// TestRuntime_IsTheInjectedBean pins that the composition root is what the
// container hands out: a starter that injects *Runtime sees the same authority
// instances the control plane pushes into.
func TestRuntime_IsTheInjectedBean(t *testing.T) {
	var rt *Runtime

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func() source.Source[resilience.TimeoutRetrySettings] {
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
		if got := timeoutOf(t, rt.Resilience, "x", chain.Operation{Service: "x"}); got != 120*time.Millisecond {
			t.Fatalf("the injected runtime must be the armed one: want 120ms, got %v", got)
		}
		if rt.Admission == nil || rt.ClientFault == nil || rt.ServerFault == nil {
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

	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	fact2, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	adm, err := admission.NewManager(fact2)
	if err != nil {
		t.Fatal(err)
	}
	rt := &Runtime{
		Resilience:  res,
		LoadBalance: loadbalance.NewManager(loadbalance.NewFactory([]loadbalance.Provider{loadbalance.NewBuiltinProvider()})),
		Admission:   adm,
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
	if rt.Directory != disc {
		t.Fatal("the runtime must hand out the discovery manager it was built with")
	}
	got, ok := rt.Directory.Get("etcd.main")
	if !ok || got != backend {
		t.Fatalf("discovery lookup: ok=%v backend=%v", ok, got)
	}
}
