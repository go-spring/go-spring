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
	"strings"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/runtime/dataplane/inbound/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	clientfault "go-spring.org/cloud/runtime/dataplane/outbound/fault"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
)

// govCfg builds a document with an outbound timeout and a CLIENT-side fault
// fire; the server side is covered by the same shape on the other block.
func govCfg(timeoutMs int, fc fault.Config) Config {
	return Config{
		Client: ClientConfig{
			Default: ClientDefaultPolicy{
				Policy: resilience.Policy{Timeout: time.Duration(timeoutMs) * time.Millisecond},
			},
			Fault: fc,
		},
	}
}

// newTestCenter builds the center bean over a fresh set of authorities, exactly
// as this package's init registers them: one instance per module, shared between
// the center and the callers that inject them. GoLive is NOT called — a caller
// that needs the center live calls it itself, the way the container does through
// the bean's Init hook.
func newTestStartup() (*Center, *resilience.Manager,
	*loadbalance.Manager, *clientfault.ClientManager, *serverfault.ServerManager) {
	return newTestStartupWith(NewPushSource(Config{}))
}

// newTestStartupWith is newTestStartup over the given source — the test's
// stand-in for what the container hands the center's constructor in production.
// The resilience authority runs on the bundled driver: the backend is a startup
// choice made at wiring, not part of the document a center test drives.
func newTestStartupWith(src Source) (*Center, *resilience.Manager,
	*loadbalance.Manager, *clientfault.ClientManager, *serverfault.ServerManager) {
	lb := loadbalance.NewManager()
	driver := resilience.NewDefaultDriver(nil)
	resClient := resilience.NewManager(driver)
	fc, fs := clientfault.NewClientManager(nil), serverfault.NewServerManager(nil)
	c, err := NewCenter(resClient, admission.NewManager(), lb, fc, fs, src)
	if err != nil {
		panic(err)
	}
	return c, resClient, lb, fc, fs
}

// TestStarter_SrcBeanArmsCenter drives the normal branch: a Source is present, so
// the center binds it at construction and GoLive distributes its snapshot into
// the module authorities — the same sequence gs runs in production.
func TestStarter_SrcBeanArmsCenter(t *testing.T) {
	_, resClient, _, fc, _ := newTestStartupWith(
		NewPushSource(govCfg(300, fault.Config{Rules: []fault.Rule{{Rate: 0.25}}})))

	if p := resClient.PolicyFor("redis:cache"); p.Timeout != 300*time.Millisecond {
		t.Fatalf("src bean policy: want 300ms, got %v", p.Timeout)
	}
	if cf := fc.Config(); len(cf.Rules) != 1 || cf.Rules[0].Rate != 0.25 {
		t.Fatal("GoLive should push the snapshot's Fault into the injected injector")
	}
}

// TestStarter_EmptySource_StaysUnarmed pins the empty-document contract: a
// source that holds the zero Config still arms the center (Init runs GoLive and
// the center goes live) but every service resolves a transparent pass-through.
// The absence of a Source bean is a different failure — the container fails
// startup on the center's required Source parameter, so there is no nil-source
// path to pin here.
func TestStarter_EmptySource_StaysUnarmed(t *testing.T) {
	_, resClient, lb, fc, fs := newTestStartup()

	if p := resClient.PolicyFor("redis:cache"); !p.IsZero() {
		t.Fatalf("empty source: resilience must resolve a zero policy, got %v", p)
	}
	// The manager exposes no read-back — production only ever pushes into sinks —
	// so ask through one: a chain registers with the selection already in force.
	lbChain := lb.Chain(context.Background(), func() ([]discovery.Endpoint, error) { return nil, nil }, "redis:cache")
	if got := lbChain.Selection(); got.Balancer != "" || got.OutlierThreshold != 0 {
		t.Fatalf("empty source: selection must resolve zero, got %+v", got)
	}
	if !fc.Config().Inert() || !fs.Config().Inert() {
		t.Fatal("empty source: fault must stay inert")
	}
}

// TestStarter_LatePushDrivesArmedModules pins the push path end to end: after
// Init, a source push reaches the module authorities with no further setup step.
func TestStarter_LatePushDrivesArmedModules(t *testing.T) {
	src := NewPushSource(govCfg(100, fault.Config{}))
	_, resClient, _, fc, _ := newTestStartupWith(src)

	// A live push swaps both the resilience side and the fault side.
	src.Push(context.Background(), govCfg(400, fault.Config{Rules: []fault.Rule{{Rate: 0.5}}}))
	if p := resClient.PolicyFor("x"); p.Timeout != 400*time.Millisecond {
		t.Fatalf("push should reach the resilience manager: want 400ms, got %v", p.Timeout)
	}
	if cf := fc.Config(); len(cf.Rules) != 1 || cf.Rules[0].Rate != 0.5 {
		t.Fatalf("push should reach the injector: %+v", cf)
	}
}

// TestStarter_UnknownBalancerFailsStartup pins the fail-loud contract for the
// selection half: a document naming a strategy this process never registered
// aborts startup, instead of silently serving chains on the base strategy while
// the operator believes their rule took effect.
func TestStarter_UnknownBalancerFailsStartup(t *testing.T) {
	driver := resilience.NewDefaultDriver(nil)
	_, err := NewCenter(resilience.NewManager(driver), admission.NewManager(), loadbalance.NewManager(),
		clientfault.NewClientManager(nil), serverfault.NewServerManager(nil),
		NewPushSource(selectionConfig("redis:cache", "no_such_strategy", 0, 0)))
	if err == nil {
		t.Fatal("an unregistered strategy name must fail construction")
	}
	if !strings.Contains(err.Error(), "no strategy registered") {
		t.Fatalf("error should name the missing strategy, got: %v", err)
	}
}
