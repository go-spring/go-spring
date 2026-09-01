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

package lock

import (
	"testing"
	"time"
)

// Resolve encodes the only timing-precedence rule the backends share, so it
// earns a direct test: each layer (per-call > starter Defaults > package
// default) must win in turn, and the special RenewInterval signs must survive.

func TestResolveZeroDefaultsMatchesApply(t *testing.T) {
	// A backend with no starter-level knobs (e.g. the k8s backend) passes a zero
	// Defaults; the timing result must be identical to plain Apply. (Token is
	// random, so compare only the deterministic fields.)
	got := Resolve(Defaults{})
	want := Apply()
	if got.TTL != want.TTL || got.RenewInterval != want.RenewInterval || got.RetryInterval != want.RetryInterval {
		t.Fatalf("zero Defaults: got %+v, want Apply() timing %+v", got, want)
	}
}

func TestResolveStarterDefaultFillsUnsetFields(t *testing.T) {
	d := Defaults{TTL: 7 * time.Second, RenewInterval: 2 * time.Second, RetryInterval: 250 * time.Millisecond}
	o := Resolve(d) // caller passes nothing
	if o.TTL != 7*time.Second {
		t.Errorf("TTL: got %v, want starter default 7s", o.TTL)
	}
	if o.RenewInterval != 2*time.Second {
		t.Errorf("RenewInterval: got %v, want starter default 2s", o.RenewInterval)
	}
	if o.RetryInterval != 250*time.Millisecond {
		t.Errorf("RetryInterval: got %v, want starter default 250ms", o.RetryInterval)
	}
}

func TestResolvePerCallOptsOverrideStarterDefaults(t *testing.T) {
	// This is the bug that was fixed: etcd used to ignore per-call TTL entirely.
	d := Defaults{TTL: 7 * time.Second, RenewInterval: 2 * time.Second, RetryInterval: 250 * time.Millisecond}
	o := Resolve(d, WithTTL(90*time.Second))
	if o.TTL != 90*time.Second {
		t.Errorf("per-call TTL must override starter default: got %v, want 90s", o.TTL)
	}
	// Untouched fields still take the starter default, not the package default.
	if o.RenewInterval != 2*time.Second {
		t.Errorf("RenewInterval should keep starter default: got %v, want 2s", o.RenewInterval)
	}
	if o.RetryInterval != 250*time.Millisecond {
		t.Errorf("RetryInterval should keep starter default: got %v, want 250ms", o.RetryInterval)
	}
}

func TestResolveStarterDefaultFallsThroughToPackageDefaultWhenZero(t *testing.T) {
	// A starter that exposes only TTL leaves renew/retry at zero, so the package
	// defaults (TTL/3 and 100ms) must fill in — exactly like etcd/consul.
	d := Defaults{TTL: 12 * time.Second}
	o := Resolve(d)
	if o.TTL != 12*time.Second {
		t.Errorf("TTL: got %v, want 12s", o.TTL)
	}
	if o.RenewInterval != 4*time.Second { // 12s / 3
		t.Errorf("RenewInterval: got %v, want package default TTL/3 = 4s", o.RenewInterval)
	}
	if o.RetryInterval != 100*time.Millisecond {
		t.Errorf("RetryInterval: got %v, want package default 100ms", o.RetryInterval)
	}
}

func TestResolvePreservesExplicitRenewDisable(t *testing.T) {
	// A caller passing WithRenewInterval(negative) disables auto-renew. The
	// starter default must NOT override that explicit intent — RenewInterval is
	// the one field where 0 and <0 carry different meanings.
	d := Defaults{RenewInterval: 2 * time.Second}
	o := Resolve(d, WithRenewInterval(-1))
	if o.RenewInterval != -1 {
		t.Errorf("explicit renew-disable clobbered by starter default: got %v, want -1", o.RenewInterval)
	}
}
