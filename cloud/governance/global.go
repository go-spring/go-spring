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

// This file holds the governance singleton and its public facade. The
// governance authority is a process singleton, but callers never hold or name a
// [*Center]: they call the package-level functions (Enabled, PolicyFor,
// Register, OnReady). [*Center] is an internal implementation detail.
// This mirrors the neutral global seams the package already exposes for
// resilience ([resilience.ExecutorFor]) and fault injection, but is the direct
// surface for callers (like starter-dubbo) that already import governance.

package governance

import (
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/governance/resilience"
)

// global is the process-wide governance authority. It is initialized to a
// disabled (zero-config) Center at package load, so it is never nil. In
// production starter-governance's wiring bean drives it in place through
// [BindDefault] and [GoLive] (this package registers no beans and imports no
// container). Tests reassign it via [Arm]/[Reset]. Because it is armed once
// during wiring (or a test) and only read after, the plain pointer needs no
// atomic or mutex — the starter establishes the happens-before edge between
// wiring and server start.
var global = NewCenter(Config{})

// live marks whether global has been armed with real config — set by
// [Center.Init] (production) or [Arm] (tests), cleared by [Reset]. [OnReady]
// queues callbacks until live, then fires them. It cannot key off global itself
// being non-nil, because global is never nil (it starts as a disabled Center).
var (
	live atomic.Bool

	// readyMu guards readyCbs, the [OnReady] callback queue. OnReady is a cold
	// path (startup registration), so a plain mutex is fine; it closes the race
	// between a late OnReady and markLive: whichever wins, each callback runs
	// exactly once — either from the queue at GoLive time or immediately.
	readyMu  sync.Mutex
	readyCbs []func()
)

// markLive flags the authority armed and fires every callback queued via
// [OnReady]. The atomic CompareAndSwap guarantees exactly-once firing even if
// GoLive is somehow invoked concurrently; callbacks run outside readyMu so a
// callback may itself call OnReady (which now fires immediately) without
// self-deadlocking.
func markLive() {
	if !live.CompareAndSwap(false, true) {
		return
	}
	readyMu.Lock()
	cbs := readyCbs
	readyCbs = nil
	readyMu.Unlock()
	for _, cb := range cbs {
		cb()
	}
}

// Enabled reports whether governance is armed. Before the authority is live it
// returns false (the singleton starts as a disabled Center) — the same as a
// registered-but-disabled authority.
func Enabled() bool { return global.Enabled() }

// Driver returns the configured resilience driver name, defaulting to "default"
// when unset (including before the authority is live).
func Driver() string { return global.Driver() }

// PolicyFor returns the resolved policy for label. Before the authority is live
// it returns a zero (pass-through) policy. It is the function callers use instead
// of holding a [*Center]: pass your resource label, get the policy.
func PolicyFor(label string) resilience.Policy { return global.PolicyFor(label) }

// Register subscribes cb to policy changes for label, arms it immediately with
// the current resolved policy, and returns that policy. This is how a caller
// subscribes to governance hot-reload without ever touching a [*Center].
func Register(label string, cb func(resilience.Policy)) resilience.Policy {
	return global.Register(label, cb)
}

// SetSource installs s as the governance config source, replacing the default
// source (or any previously set source). It is the custom-source entry point
// for push-based integrations — a governance console, a dedicated
// config-center listener, a static injection. Call it any time:
//
//   - before the wiring starter binds its default: the center arms from s and
//     the default never takes over (BindDefault is a no-op when a source is
//     already bound);
//   - afterwards: late-arm — s.Snapshot() applies immediately and subsequent
//     pushes drive the center; the previous source's callbacks go stale.
//
// A bean route exists too: gs.Provide(newSrc).Export(gs.As[Source]()) is
// field-injected onto starter-governance's wiring bean (a missing bean leaves
// the default path). The Export is NOT optional for that route — without it
// the bean is invisible to interface injection and governance silently runs
// on the default again.
//
// Source priority is SetSource > injected bean > the wiring default. Exactly
// one source is active at a time; the center does not merge — [Config.Rules]
// is a whole-replace model, so a merge policy is a decision that belongs
// inside a composite Source implementation, not in the center. Removing a
// custom source is not supported; restart instead.
func SetSource(s Source) { global.setSource(s) }

// BindDefault installs src as the active source only when none is bound yet —
// an explicit SetSource (callable at any time) always outranks it. This is the
// one-shot hook the wiring starter calls at startup with its chosen default
// (a bean-injected Source, else the ${govern} Dync adapter).
func BindDefault(src Source) { global.bindDefault(src) }

// GoLive completes the authority's startup: it builds the process-wide fault
// injector from the current snapshot, registers the executor/fault seams, and
// marks the authority live (firing any [OnReady] callbacks). Idempotent. The
// wiring starter calls it once after BindDefault.
func GoLive() { global.goLive() }

// CloseActiveSource closes the active source when it happens to implement
// Close (the [Source] contract keeps Close optional); otherwise it is a no-op.
// It is the shutdown counterpart to [BindDefault]/[GoLive], called by the
// wiring starter on app teardown.
func CloseActiveSource() error { return global.Destroy() }

// OnReady registers cb to fire exactly once when the authority goes live. If it
// is already live, cb fires immediately. gs wires Rooters before Runners, so a
// push-based caller (starter-dubbo, a Rooter) may initialize before the wiring
// starter (also a Rooter) has armed governance; OnReady guarantees the caller
// re-runs its work once governance is live, without depending on bean order.
func OnReady(cb func()) {
	if live.Load() {
		cb()
		return
	}
	readyMu.Lock()
	if live.Load() { // went live while we waited on the lock: fire now
		readyMu.Unlock()
		cb()
		return
	}
	readyCbs = append(readyCbs, cb)
	readyMu.Unlock()
}

// Arm installs an authority built from cfg onto the singleton and returns a
// reset function that restores the disabled default. It is the non-gs wiring
// path: tests (and any standalone use without gs) drive governance through it.
func Arm(cfg Config) (reset func()) {
	global = NewCenter(cfg)
	markLive()
	return Reset
}

// Reset restores the singleton to a disabled (zero-config) authority and clears
// the live flag. It is the cleanup counterpart to [Arm].
func Reset() {
	global = NewCenter(Config{})
	readyMu.Lock()
	live.Store(false)
	readyCbs = nil
	readyMu.Unlock()
}
