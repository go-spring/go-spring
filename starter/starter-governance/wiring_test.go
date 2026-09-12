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
	"testing"
	"time"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

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
