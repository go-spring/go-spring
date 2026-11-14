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

// limits.go is the limits capability: the pushed document (timeout and retry
// schedule per service/interface), its validation, the matching rules that
// resolve a call to the policy it runs under, and the retry-classification
// vocabulary errors opt into ([Retryable], [ShouldRetry]).

package resilience

import (
	"errors"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// Retry is the retry schedule of the GOVERNANCE document: how many extra
// attempts are allowed, how they are paced, and the two safety knobs around
// them. It is configured per service/interface by the management platform and
// read from the policy — there is no call-site declaration, so a call that must
// not be retried says so in the pushed document (no schedule for its
// service/interface), not in its own signature.
//
// Retry is the one protection stage that is not a gate but a control-flow loop,
// and the only one that can AMPLIFY a downstream's load precisely when the
// downstream is least able to take it. That is why the zero value means "no
// retry": a call runs exactly once unless the document declares a schedule.
//
// The node is a comparable value struct on purpose: it is embedded in the
// policy, which the manager compares with != to detect a changed policy on hot
// reload. A pointer or interface field would silently degrade that comparison to
// pointer identity. Zero value == no retry also means there is no "declared but
// disabled" state to represent, so a value loses nothing.
//
// The value tags make this node directly bindable as a governance document half
// (config-policy documents bind through the same value-tag machinery), so a
// center expresses retry with keys like services[0].rules[0].retry.max-retries.
type Retry struct {
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

	// RetryBudget caps the number of retry attempts in flight across all services
	// served by one executor at the same time — the process-level guard against
	// retry amplification. An over-budget retry is rejected with
	// [chain.ErrRetryExceeded] (the first attempt always runs; only retries take
	// budget). 0 disables the cap.
	RetryBudget int `value:"${retry-budget:=0}" expr:"$ >= 0"`
}

// IsZero reports whether the node declares nothing at all. Retry is comparable,
// so this is just a comparison against the zero value — which also means a
// newly added field cannot be forgotten here.
func (p Retry) IsZero() bool {
	return p == Retry{}
}

// TimeoutRetry is a policy's timeout and retry half: the wall-time budget of one
// Execute and the schedule its rounds follow. It carries no runtime state — the
// executor reads these values per call.
type TimeoutRetry struct {
	// Retry is the retry schedule for this label. It is the ONLY source of
	// retry: call-site code cannot declare one, so a call whose repetition is a
	// second side effect (sending a mail, publishing a message) is protected by
	// the document simply declaring no schedule for its service/interface. The
	// zero node means "no retry" — the call runs exactly once.
	//
	// RetryBudget is read from HERE only, at the label level (see
	// [Retry.RetryBudget]).
	Retry Retry `value:"${retry:=}"`

	// Timeout bounds the WALL TIME of the whole [chain.Executor.Execute] — retries
	// and backoff included — as one context deadline applied at the entry and
	// covering every stage beneath it.
	//
	// It is deliberately ONE bound rather than a per-attempt one plus a total: a
	// single deadline is what an operator reasons about ("this call must not
	// exceed N"), and it is what the governance layers of every mainstream stack
	// express. Its consequence is that rounds share the budget — a first attempt
	// that spends it all leaves nothing for the retries — which is the same
	// contract Istio's route timeout and gRPC's deadline carry.
	//
	// 0 means no bound of its own, beyond whatever the caller's context carries.
	Timeout time.Duration `value:"${timeout:=0}" expr:"int($) >= 0"`

	// ConnectTimeout bounds ESTABLISHING the connection within a round — the
	// dial and the pool wait that precedes it — independently of Timeout. It is
	// the fast-fail knob for a blackholed peer (a refused connection answers
	// immediately; only dropped packets eat the whole budget) and the transport
	// counterpart mainstream drivers carry beside their call timeout
	// (connectTimeout vs socketTimeout). It travels to the dial site on the
	// round's context ([chain.RoundFrom]) and applies as
	// min(connect-timeout, the site's own static cap, the remaining round
	// budget) — it can only tighten, never widen.
	//
	// 0 means no bound of its own: the dial runs under the site's static cap
	// and the round's remaining Timeout budget.
	ConnectTimeout time.Duration `value:"${connect-timeout:=0}" expr:"int($) >= 0"`
}

// IsZero reports whether these limits configure nothing — no deadline and no
// schedule, so the call runs on its own context and its own retry node.
func (l TimeoutRetry) IsZero() bool {
	return l == TimeoutRetry{}
}

// TimeoutRetrySettings is the timeout/retry half of the outbound document, PUSHED whole: the
// center hands it over and this package does the matching. This type IS the
// document — the control-plane limits root binds it and hands it back unchanged
// (see [TimeoutRetrySettings.Validate]) — so there is no separate binding twin.
//
// It carries values only, never the backend: which [Driver] builds the executors
// is a startup decision, not part of the tunable document.
//
// The zero settings configure nothing: every label resolves zero limits, which
// every caller reads as "no deadline, no schedule". There is no arming flag — an
// empty document is a pass-through, and so is an absent one.
type TimeoutRetrySettings struct {
	// Services groups the rules by the label they govern. Duplicate groups are
	// order-shadowed: the first group naming a label wins. There is no implicit
	// default: a label with no matching rule gets no deadline and no schedule.
	Services []TimeoutRetryService `value:"${services:=}"`
}

// TimeoutRetryService is one label's policy block: every rule inside it governs
// the same service. Among real-action rules the most specific match wins (exact
// beats any prefix, a longer prefix beats a shorter one, a larger MetadataMatch
// narrows further); a service-scope rule is the fallback when no real action
// matches. Two rules that tie on specificity are decided by list order —
// avoiding that is the configuration platform's job, not a check here.
type TimeoutRetryService struct {
	// Service is the label these rules apply to, exact-compare. Required.
	Service string `value:"${service:=}" expr:"$ != ''"`

	// Rules are the limits for this service.
	Rules []TimeoutRetryRule `value:"${rules:=}"`
}

// Validate reports whether s can serve. conf.Bind calls it, so an invalid
// document is rejected at the source boundary rather than reaching the center.
//
// TimeoutRetry rules accept both scopes: a [matcher.ServiceScopeAction] rule (no MetadataMatch, may carry a
// label-scoped retry budget) and real-action rules (MetadataMatch allowed, budget
// rejected — a budget is per label). A real action always wins over the service
// marker. What two equally specific rules would do to a call is not checked
// here: the platform generating the document is expected not to emit overlap.
func (s TimeoutRetrySettings) Validate() error {
	for i := range s.Services {
		g := &s.Services[i]
		for j := range g.Rules {
			r := &g.Rules[j]
			if r.ActionMatch.IsServiceScope() {
				if len(r.MetadataMatch) != 0 {
					return errutil.Explain(nil, "resilience: a service-scope rule for %q must not carry a match selector", g.Service)
				}
			} else if r.Retry.RetryBudget != 0 {
				return errutil.Explain(nil, "resilience: retry-budget is label-scoped and cannot be set on the action rule %q of %q",
					r.ActionMatch, g.Service)
			}
		}
	}
	return nil
}

// TimeoutRetryRule is one timeout/retry rule inside a [TimeoutRetryService].
//
// Action is either [matcher.ServiceScopeAction] ("-"), meaning the whole service, or a
// real exact/prefix selector, meaning one interface. A real action always wins
// over the service marker. MetadataMatch further narrows a real-action rule; the
// service marker does not accept one (a service-level rule already covers the label).
type TimeoutRetryRule struct {
	// Action selects the scope: [matcher.ServiceScopeAction] for the whole service, or a
	// real exact/prefix.
	ActionMatch matcher.ActionMatch `value:"${action:=}"`

	// MetadataMatch narrows a real-action rule by call metadata (subset-match):
	// one [matcher.StringMatch] per key, the op always spelled out. Must be
	// empty on a [matcher.ServiceScopeAction] rule. The config key stays
	// `match.` (e.g. match.tenant.exact=core), the same prefix the fault rules
	// use.
	MetadataMatch map[string]matcher.StringMatch `value:"${match:=}"`

	TimeoutRetry
}

// Retryable is implemented by an error to opt itself in or out of retry: the
// error's own type knows best whether another attempt is meaningful (a
// non-idempotent write that already failed server-side opts out; a transient
// reset opts in). This is how a gorm/redis adapter marks a
// non-idempotent-write error (Retryable() false) or a transient one (true)
// without teaching the executor about that client library.
type Retryable interface {
	Retryable() bool
}

// ShouldRetry reports whether err ought to trigger another attempt. Retry is a
// WHITELIST: only an error that implements [Retryable] and answers true is
// retried — every other error runs once and surfaces, because an unclassified
// failure is not known to be transient and a retry may repeat a side effect.
// Client adapters know their own transient errors (a timeout, a connection
// reset, a 5xx) and opt them in here; the framework can never enumerate a
// third-party library's failures, so the default must be "no".
func ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var r Retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return false
}

// findRule answers the timeout/retry half of the call plan ([Executor.planFor]):
// a real-action rule always beats the service marker; among real actions the
// most specific match wins. A tie between equally specific rules is broken by
// list order — the platform's document is expected not to create one.
//
// The second result reports whether a rule matched. Timeout and retry keep no
// per-rule state, so there is no key to carry.
//
// The returned rule's schedule carries no retry budget: the budget is a
// label-level fact and is read through [TimeoutRetrySettings.labelBudget], so
// the per-call schedule never varies with it.
func (s *TimeoutRetrySettings) findRule(op chain.Operation) (TimeoutRetryRule, bool) {
	for i := range s.Services {
		if s.Services[i].Service == op.Service {
			// The first group naming a label wins; a duplicate group is
			// order-shadowed, not merged.
			return s.Services[i].findRule(op)
		}
	}
	return TimeoutRetryRule{}, false
}

// findRule selects one rule of this service.
func (s *TimeoutRetryService) findRule(op chain.Operation) (TimeoutRetryRule, bool) {
	best := -1
	var bestMatch matcher.ActionMatch
	bestMetadata := 0
	for i := range s.Rules {
		r := &s.Rules[i]
		if !r.ActionMatch.Matches(op.Request.Action) {
			continue
		}
		// A service-scope rule is only the fallback: it ranks below every real
		// action.
		if r.ActionMatch.IsServiceScope() {
			if best < 0 {
				best = i
			}
			continue
		}
		if !matcher.MetadataMatches(r.MetadataMatch, op.Request.Metadata) {
			continue
		}
		// Action specificity decides first; a larger metadata selector breaks a
		// tie between equally specific actions.
		metaLen := len(r.MetadataMatch)
		if best < 0 || r.ActionMatch.BetterThan(bestMatch) ||
			(!bestMatch.BetterThan(r.ActionMatch) && metaLen > bestMetadata) {
			best, bestMatch, bestMetadata = i, r.ActionMatch, metaLen
		}
	}

	if best < 0 {
		return TimeoutRetryRule{}, false
	}
	rule := s.Rules[best]
	rule.Retry.RetryBudget = 0
	return rule, true
}

// labelBudget is the label-level retry cap: the service-scope rule's budget.
// It caps retries in flight across the whole label, so it must be one number —
// and an action rule that omits it (the common case) must not silently disable
// the cap. No service-scope rule means no cap.
func (s *TimeoutRetrySettings) labelBudget(service string) int {
	for i := range s.Services {
		g := &s.Services[i]
		if g.Service != service {
			continue
		}
		for j := range g.Rules {
			if g.Rules[j].ActionMatch.IsServiceScope() {
				return g.Rules[j].Retry.RetryBudget
			}
		}
		return 0 // first group naming the label wins, as in findRule
	}
	return 0
}
