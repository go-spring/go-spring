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
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// setDyncGov pushes cfg into the wiring's Gov field, standing in for what gs
// field injection does in a live app (gs.Dync offers no public setter; same
// unsafe trick as starter-dubbo's dync_test.go).
func setDyncGov(w *wiring, cfg governance.Config) {
	type dync struct{ v atomic.Value }
	(*dync)(unsafe.Pointer(&w.Gov)).v.Store(cfg)
}

// govCfg builds a governance.Config with the given default attempt timeout and
// an armed fault section.
func govCfg(timeoutMs int, fc fault.Config) governance.Config {
	return governance.Config{
		Enabled: true,
		Default: resilience.PolicyConfig{AttemptTimeout: time.Duration(timeoutMs) * time.Millisecond},
		Fault:   fc,
	}
}

// TestWiring_DefaultDyncPath drives the wiring bean's default branch directly:
// no custom source, so Init binds the dyncSource adapter over the ${govern}
// Dync and GoLive arms the center from its snapshot — the same sequence gs
// runs on the wiring bean in production.
//
// It cannot go through gs.RunTest: gs_init.Beans() CLONES global bean
// definitions under testing.Testing() (test isolation), so a test binary wires
// a copy of the bean while the governance facade still reads the package
// singleton. Driving Init directly covers the same code path without that gap.
func TestWiring_DefaultDyncPath(t *testing.T) {
	defer governance.Reset()

	w := newWiring()
	setDyncGov(w, govCfg(100, fault.Config{Enabled: true, Rate: 0.25}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}

	if !governance.Enabled() {
		t.Fatal("wiring should arm the center from the ${govern} snapshot")
	}
	if p := governance.PolicyFor("redis:cache"); p.Timeout != 100*time.Millisecond {
		t.Fatalf("default path policy: want 100ms, got %v", p.Timeout)
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

// TestWiring_SrcBeanWinsOverDync covers the bean-injection route: a Source
// present on the wiring struct outranks the ${govern} Dync.
func TestWiring_SrcBeanWinsOverDync(t *testing.T) {
	defer governance.Reset()

	w := newWiring()
	setDyncGov(w, govCfg(100, fault.Config{})) // would arm 100ms if consulted
	w.Src = governance.NewPushSource(govCfg(300, fault.Config{}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}
	if p := governance.PolicyFor("x"); p.Timeout != 300*time.Millisecond {
		t.Fatalf("Src bean should win over the Dync default: want 300ms, got %v", p.Timeout)
	}
}

// TestWiring_ExplicitSetSourceWinsOverAll covers the top of the priority
// chain: a SetSource called before wiring pre-empts both the Src bean and the
// Dync default (BindDefault is a no-op when a source is already bound).
func TestWiring_ExplicitSetSourceWinsOverAll(t *testing.T) {
	defer governance.Reset()

	governance.SetSource(governance.NewPushSource(govCfg(200, fault.Config{})))

	w := newWiring()
	setDyncGov(w, govCfg(100, fault.Config{}))
	w.Src = governance.NewPushSource(govCfg(300, fault.Config{}))
	if err := w.Init(); err != nil {
		t.Fatal(err)
	}
	if p := governance.PolicyFor("x"); p.Timeout != 200*time.Millisecond {
		t.Fatalf("explicit SetSource should win: want 200ms, got %v", p.Timeout)
	}
}
