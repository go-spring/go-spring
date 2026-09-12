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
	"slices"
	"time"
)

// Config binds the fault-injection knobs from go-spring ${...} value tags. It is
// embedded in [governance.Config] (field tag `${fault:=}`, so its keys bind as
// govern.fault.*) and shares that single binding with resilience — there is no
// per-starter fault key anymore. A zero
// Config (Enabled false) injects nothing; the process-wide injector built from
// it is a transparent no-op.
//
// The top-level Rate/Latency/Error apply to every resource; per-resource
// overrides go under Rules (matched by the resource label a starter passes to
// the executor/Apply seam).
type Config struct {
	// Enabled turns fault injection on. When false no faults are injected even
	// if Rate/Latency/Error are set.
	Enabled bool `value:"${enabled:=false}"`

	// Rate is the per-call probability (0..1) of injecting the configured Error.
	// 0 means never inject; 1 means every call fails. Latency, when set, is
	// applied to every call regardless of Rate — use it to model a uniformly
	// slow downstream without forcing errors.
	Rate float64 `value:"${rate:=0}"`

	// Latency is injected before each call (and before the error decision). Use
	// it to exercise slow-call and per-attempt timeout paths. 0 disables.
	Latency time.Duration `value:"${latency:=0}"`

	// Error selects the injected failure kind, applied at Rate:
	//   "" / "generic" — a retryable injected error ([ErrInjected])
	//   "timeout"      — wraps context.DeadlineExceeded
	//   "reset"        — wraps syscall.ECONNRESET
	// Empty (the default) plus Rate 0 means no errors are injected.
	Error string `value:"${error:=}"`

	// Scope restricts fault injection to a class of traffic identified by the
	// load-test marker carried in the call's context ([traffic.IsLoadTest]):
	//   ""        — inject into all traffic (the default; the historic behavior)
	//   "real"    — inject only into real traffic, skipping load-test requests
	//               (so synthetic load from cloud/loadtest does not get faulted)
	//   "loadtest"— inject only into load-test traffic (so faults exercise the
	//               resilience stack under synthetic load without disturbing
	//               production traffic)
	// An unknown value behaves as "".
	Scope string `value:"${scope:=}"`

	// MaxDuration is the safety auto-off: after this much time elapses since the
	// first affected call, the injector stops applying any fault (latency or
	// error). It lets an operator enable a fault and walk away — a forgotten
	// fire self-heals rather than running until the next deploy. 0 (the default)
	// means no time bound. Hot-reload note: shortening MaxDuration takes effect
	// promptly; lengthening it does not un-trip an already-expired fire (re-arm
	// by toggling Enabled off and back on).
	MaxDuration time.Duration `value:"${max-duration:=0}"`

	// MaxAffected is the safety blast-radius cap: once this many calls have
	// received any fault effect (latency or error), the injector stops. Use it
	// to bound the impact of a high-Rate fire on production traffic. 0 (the
	// default) means no count bound.
	MaxAffected int64 `value:"${max-affected:=0}"`

	// Rules are per-resource overrides. A call whose resource label matches a
	// rule (exact match against any of the rule's Resources, or the rule itself
	// when Resources is empty — a catch-all) uses that rule's Rate/Latency/Error
	// instead of the top-level ones. The first matching rule wins, so list
	// specific rules before a catch-all. When no rule matches, the top-level
	// Rate/Latency/Error apply (so adding Rules only adds specificity, never
	// removes the global default). Empty (the default) keeps the historic
	// single-rule behavior. Bind via indexed properties:
	//
	//	fault.rules[0].resources=svc-a,svc-b
	//	fault.rules[0].rate=0.5
	//	fault.rules[1].rate=1   # empty resources => catch-all
	Rules []Rule `value:"${rules:=}"`
}

// Rule is one per-resource fault rule. See [Config.Rules].
type Rule struct {
	// Resources are the resource labels this rule matches, exact-compare. Empty
	// means catch-all (matches every resource). A resource label is what a
	// starter passes to the executor/fault seam — e.g. "redis", "gorm:mysql",
	// "http:user-svc", "gin", a downstream service name.
	Resources []string      `value:"${resources:=}"`
	Rate      float64       `value:"${rate:=0}"`
	Latency   time.Duration `value:"${latency:=0}"`
	Error     string        `value:"${error:=}"`
}

// matches reports whether r applies to resource: catch-all when r has no
// Resources, else exact membership.
func (r Rule) matches(resource string) bool {
	if len(r.Resources) == 0 {
		return true
	}
	return slices.Contains(r.Resources, resource)
}
