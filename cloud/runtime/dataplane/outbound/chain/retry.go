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

package chain

import "time"

// RetryPolicy is the retry declaration for one call: how many extra attempts
// are allowed, how they are paced, and the two safety knobs around them. It is
// the explicit CHANNEL through which retry is configured — it travels on
// [Operation], declared by the caller that owns the call, and may be overridden
// whole by a governance center's policy (see the resilience package's
// EffectiveRetry).
//
// Retry is the one protection stage that is not a gate but a control-flow loop,
// and the only one that can AMPLIFY a downstream's load precisely when the
// downstream is least able to take it. That is why it is deliberately NOT a
// framework default: the zero value means "no retry" — a call runs exactly once
// unless someone declares otherwise. The caller is the only side that knows
// whether repeating the call repeats a side effect, so the caller is the one who
// opts in.
//
// The node is a comparable value struct on purpose: it is embedded in the
// resilience policy, which the manager compares with != to detect a changed
// policy on hot reload. A pointer or interface field would silently degrade that
// comparison to pointer identity. Zero value == no retry also means there is no
// "declared but disabled" state to represent, so a value loses nothing.
//
// The value tags make this node directly bindable as a governance document half
// (config-policy documents bind through the same value-tag machinery), so a
// center expresses retry with keys like
// spring.governance.client.rules[0].retry.max-retries. The tag strings are the
// only place this package names config keys; the coupling is intentional — a
// document's expressible retry IS this type, with no binding twin.
type RetryPolicy struct {
	// MaxRetries is the number of extra attempts after the first failure. 0 (the
	// zero value) means a single attempt with no retry.
	MaxRetries int `value:"${max-retries:=0}" expr:"$ >= 0"`

	// InitialInterval is the backoff before the first retry. 0 disables backoff
	// entirely (retries run back-to-back). Pair with Multiplier, MaxInterval and
	// RandomizationFactor.
	InitialInterval time.Duration `value:"${initial-interval:=0}" expr:"int($) >= 0"`

	// Multiplier is the exponential growth factor applied to InitialInterval
	// between successive retries. 0 and 1 both mean a constant interval.
	Multiplier float64 `value:"${multiplier:=0}" expr:"$ >= 0"`

	// MaxInterval caps the grown backoff so it does not expand unbounded. 0 means
	// no cap.
	MaxInterval time.Duration `value:"${max-interval:=0}" expr:"int($) >= 0"`

	// RandomizationFactor is the jitter fraction in [0,1) applied to each computed
	// interval as interval*(1 ± factor), decorrelating clients that would
	// otherwise retry in lockstep. 0 means no jitter.
	RandomizationFactor float64 `value:"${randomization-factor:=0}" expr:"$ >= 0 && $ < 1"`

	// NonIdempotent marks a call whose repetition is a second side effect rather
	// than a second attempt — sending an email, publishing to a broker. A call
	// declaring it runs once whatever retry the schedule asks for, because a
	// retry would repeat the side effect rather than the attempt.
	//
	// It is a property of the CALL, not of the schedule, and is OR-ed across a
	// policy node and a call node when the two are merged: a governance override
	// that replaces the whole node must never silently drop a caller's
	// non-idempotent declaration and re-enable retry on a side-effecting call.
	NonIdempotent bool `value:"${non-idempotent:=false}"`

	// RetryBudget caps the number of retry attempts in flight across all services
	// served by one executor at the same time — the process-level guard against
	// retry amplification. An over-budget retry is rejected with
	// ErrRetryBudgetExceeded (the first attempt always runs; only retries take
	// budget). 0 disables the cap.
	//
	// It is process-level state, sized once when the executor is built, so it is
	// read ONLY from a policy node at construction time; a value on a per-call
	// node is deliberately ignored (a per-call budget would not be shared, which
	// is the whole point of the guard).
	RetryBudget int `value:"${retry-budget:=0}" expr:"$ >= 0"`
}

// IsZero reports whether the node declares nothing at all.
func (p RetryPolicy) IsZero() bool {
	return p.MaxRetries == 0 && p.InitialInterval == 0 && p.Multiplier == 0 &&
		p.MaxInterval == 0 && p.RandomizationFactor == 0 &&
		!p.NonIdempotent && p.RetryBudget == 0
}

// HasSchedule reports whether the node actually declares a retry SCHEDULE — a
// count or a pacing. It deliberately ignores RetryBudget and NonIdempotent: a
// process-level budget alone, or a safety flag alone, is not a retry
// declaration, and must not override a call's own retry node when a policy and
// a call are merged (a center that pushes only a retry-budget must not wipe the
// caller's schedule).
func (p RetryPolicy) HasSchedule() bool {
	return p.MaxRetries != 0 || p.InitialInterval != 0 || p.Multiplier != 0 ||
		p.MaxInterval != 0 || p.RandomizationFactor != 0
}
