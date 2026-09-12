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

package fault

import "sync/atomic"

// This file is the fault-injection companion to resilience/provider.go. Where
// [resilience.ExecutorFor] gives clients a neutral way to obtain a governance-backed
// executor WITHOUT naming cloud/governance, [InjectorFor] gives them the same neutral
// access to the process-wide fault injector — so fault config, like resilience config,
// flows from the single [governance.Config] (and so the same source) rather than from a
// per-starter fault config.
//
// Unlike the resilience seam (which memoizes one executor per resource label), fault
// resolves to ONE process-global injector: per-resource differences are expressed inside
// the injector's own [Config.Rules], and the resource label is consulted at decision time
// by the injector (in maybe) via WrapExecutor/Apply — not at lookup time. The
// MaxDuration/MaxAffected guardrails are therefore process-global counters; see
// cloud/governance/DESIGN_CN.md §8 for the rationale.

// injectorHolder is the process-wide injector registered once by starter-govern after it
// builds the governance center. nil (the zero value) means fault injection is off for the
// whole process — [WrapExecutor] and [Apply] treat a nil injector as a transparent
// pass-through, so a process that does not import starter-govern (or has governance off)
// pays nothing.
var injector atomic.Pointer[Injector]

// RegisterInjector installs in as the process-wide fault injector. starter-govern calls
// this once after building the governance center; clients never call it. Calling more than
// once replaces the injector (the common case is a single registration). It is safe to call
// concurrently with [InjectorFor]; the injector is read via an atomic pointer load.
//
// Pass nil to disarm (subsequent InjectorFor calls return nil → pass-through).
func RegisterInjector(in *Injector) {
	injector.Store(in)
}

// InjectorFor returns the process-wide fault injector, or nil when none is registered
// (starter-governance not imported, or governance disabled). It is the single call site
// clients use instead of building their own injector from their own config: pass
// the result straight to [WrapExecutor] (client side) or [Apply] (server side); both are
// nil-safe. Hot-reload is handled by the center, which swaps the injector's config in
// place via [Injector.SetConfig], so callers always observe the latest config on the next
// call without re-resolving.
func InjectorFor() *Injector {
	return injector.Load()
}
