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

import (
	"time"
)

// Config binds the fault-injection knobs from go-spring ${...} value tags. The
// SAME type serves both traffic directions: [Injector] holds one per direction
// ([Configs]), fed from governance.Config's two direction blocks — the client
// side from spring.governance.client.fault.*, the server side from spring.governance.server.fault.*.
// There is no per-starter fault key.
//
// One type for both is deliberate: the model is direction-symmetric (a fraction
// of calls fails or slows down) and only the values differ, so two structurally
// identical types would buy nothing but conversions. Direction is expressed
// where a reader must pick one — the config block, and the entry point
// ([WrapClientExecutor] for outbound, [ApplyServer] for inbound).
//
// A zero Config injects nothing — no rate, no latency — so a side built from it
// is a transparent no-op (see [Config.Inert]).
//
// The top-level Rate/Latency/Error apply to every service of that ONE direction;
// per-service overrides go under Rules (matched by the service label a starter
// passes to the executor/ApplyServer seam).
type Config struct {
	// Rate is the per-call probability (0..1) of injecting the configured Error.
	// 0 means never inject; 1 means every call fails. Latency, when set, is
	// applied to every call regardless of Rate — use it to model a uniformly
	// slow downstream without forcing errors.
	Rate float64 `value:"${rate:=0}"`

	// Latency is injected before each call (and before the error decision). Use
	// it to exercise slow-call and per-attempt timeout paths. 0 disables.
	Latency time.Duration `value:"${latency:=0}"`

	// LatencyJitter randomizes the injected latency: each call sleeps Latency
	// plus a uniform draw in [-LatencyJitter, +LatencyJitter], clamped to ≥ 0.
	// A fixed Latency can resonate with a fixed retry backoff; jitter spreads
	// the slow-down the way real network variance does. 0 (the default) keeps
	// the sleep exactly Latency.
	LatencyJitter time.Duration `value:"${latency-jitter:=0}"`

	// Error selects the injected failure kind, applied at Rate:
	//   "" / "generic" — a retryable injected error ([ErrInjected])
	//   "timeout"      — wraps context.DeadlineExceeded
	//   "reset"        — wraps syscall.ECONNRESET
	//   "refused"      — wraps syscall.ECONNREFUSED
	// Empty (the default) plus Rate 0 means no errors are injected.
	Error string `value:"${error:=}"`

	// Scope restricts fault injection to a class of traffic identified by the
	// load-test marker carried in the call's context (the injected
	// [traffic.Propagator]'s IsLoadTest):
	//   ""        — inject into all traffic (the default; the historic behavior)
	//   "real"    — inject only into real traffic, skipping load-test requests
	//               (so synthetic load from cloud/experimental/loadtest does not get faulted)
	//   "loadtest"— inject only into load-test traffic (so faults exercise the
	//               resilience stack under synthetic load without disturbing
	//               production traffic)
	// An unknown value behaves as "".
	Scope string `value:"${scope:=}"`

	// MaxDuration is the safety auto-off: after this much time elapses since the
	// first affected call, the injector stops applying any fault (latency or
	// error). It lets an operator arm a fault and walk away — a forgotten fire
	// self-heals rather than running until the next deploy. 0 (the default)
	// means no time bound. Hot-reload note: shortening MaxDuration takes effect
	// promptly; lengthening it does not un-trip an already-expired fire — the
	// first-affected stamp is taken once per side and a config push does not
	// reset it, so re-arming the window takes a process restart.
	MaxDuration time.Duration `value:"${max-duration:=0}"`

	// MaxAffected is the safety blast-radius cap: once this many calls have
	// received any fault effect (latency or error), the injector stops. Use it
	// to bound the impact of a high-Rate fire on production traffic. 0 (the
	// default) means no count bound.
	MaxAffected int64 `value:"${max-affected:=0}"`

	// Rules are per-service overrides — one service per Rule, no grouping
	// several labels into one entry (they would silently share an injection
	// profile). A call whose service label matches a rule (exact match, or the
	// rule itself when Service is empty — a catch-all) uses that rule's
	// Rate/Latency/Error instead of the top-level ones. The first matching rule
	// wins, so list specific rules before a catch-all. When no rule matches, the
	// top-level Rate/Latency/Error apply (so adding Rules only adds specificity,
	// never removes the global default). Empty (the default) keeps the historic
	// single-rule behavior. Bind via indexed properties — the same shape on
	// either side, under that side's block, e.g.:
	//
	//	spring.governance.client.fault.rules[0].service=redis:cache
	//	spring.governance.client.fault.rules[0].rate=0.5
	//	spring.governance.server.fault.rules[0].rate=1   # empty service => catch-all inbound
	Rules []Rule `value:"${rules:=}"`
}

// Inert reports whether c can inject nothing at all: no global rate or latency,
// and no rule carrying one. It is the derived replacement for the former
// Enabled switch — Rate and Latency are the only knobs that cause an effect
// (Error needs Rate > 0, LatencyJitter needs Latency > 0, and the safety
// guardrails only ever STOP an effect), so "no rate and no latency" is exactly
// "no effect is possible". A zero Config is therefore inert, and a side built
// from it stays a transparent no-op.
//
// It is the hot path's early-out (see [Injector.gate]): O(1) when Rules is
// empty, which is the common case. Exported so the governance center can report
// whether a side is armed.
func (c Config) Inert() bool {
	if c.Rate > 0 || c.Latency > 0 {
		return false
	}
	for _, r := range c.Rules {
		if r.Rate > 0 || r.Latency > 0 {
			return false
		}
	}
	return true
}

// Rule is one per-service fault rule. See [Config.Rules].
type Rule struct {
	// Service is the service label this rule matches, exact-compare. Empty
	// means catch-all (matches every service). A service label is what a
	// starter passes to the executor/fault seam — e.g. "redis", "gorm:mysql",
	// "http:user-svc", "gin", a downstream service name.
	Service string        `value:"${service:=}"`
	Rate    float64       `value:"${rate:=0}"`
	Latency time.Duration `value:"${latency:=0}"`

	// LatencyJitter randomizes this rule's Latency; see [Config.LatencyJitter].
	LatencyJitter time.Duration `value:"${latency-jitter:=0}"`
	Error         string        `value:"${error:=}"`
}

// matches reports whether r applies to the service label: catch-all when r has
// an empty Service, else exact compare.
func (r Rule) matches(service string) bool {
	return r.Service == "" || r.Service == service
}
