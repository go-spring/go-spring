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

// policy_source.go is the PULL seam between the manager and the executors it
// built: what an executor asks its manager on every call, and how the manager
// answers without taking a lock or touching its registry.
//
// It replaces the push side of the old protocol. A policy change used to arrive
// at the executor through [Refreshable]/[ServerEngine.Refresh], which made the
// manager walk its registrations and retune every executor in place. Now a push
// stops at the manager: an Apply publishes its CAPABILITY's snapshot, and each
// executor notices on its next call that the generation it was built from is
// gone and rebuilds only the organ whose knobs moved.
//
// The generation is what makes that safe to do lazily, and there is one per
// capability. A change to one service's breaker threshold must not reset that
// service's rate budget, so each capability is resolved and fingerprinted
// independently.

package resilience

import (
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
	"time"

	"go-spring.org/cloud/governance/capability/circuitbreaker"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// LimitsRef is the limits capability as one call resolves it: the timeout and
// retry schedule the call runs under, and the generation of the limits half of
// its label's rules. Limits are pure values — nothing is materialized, so the
// generation only tells a reader whether the value moved.
type LimitsRef struct {
	Timeout time.Duration
	Retry   chain.RetryPolicy
	Gen     uint64
}

// BreakerRef is the breaker capability as one call resolves it: the policy whose
// breaker knobs apply, the key the breaker state is partitioned by, and the
// generation of the breaker half of its label's rules.
type BreakerRef struct {
	Policy BreakerPolicy
	Key    string
	Gen    uint64
}

// RateRef is the rate-limit capability as one call resolves it: the policy whose
// rate knobs apply, the key the budget is partitioned by, and the generation of
// the rate-limit half of its label's rules.
type RateRef struct {
	Policy RateLimitPolicy
	Key    string
	Gen    uint64
}

// CallPolicy is one outbound call's protection as its manager resolves it,
// split by capability. Each capability carries its OWN generation, so an
// executor rebuilds exactly the organ whose configuration moved.
//
// LabelLimits is the LIMITS half of the executor's label — the first rule naming
// it, or the default. The retry budget is a per-label guard, so it is read from
// here rather than from the rule this call matched.
//
// Gen is the three capability generations mixed into one, for a backend that
// reloads every family together (sentinel) and therefore cannot use them
// separately.
type CallPolicy struct {
	Limits      LimitsRef
	Breaker     BreakerRef
	Rate        RateRef
	LabelLimits Limits
	Gen         uint64
}

// snapshot is one capability's published rules, read lock-free by every
// executor's Resolve. The per-label generation is memoized here rather than
// precomputed, because the set of labels in use is discovered lazily — a label
// exists when some client first asks for it — so it cannot be enumerated at
// apply time. A fresh snapshot starts with an empty cache, so a memo can never
// outlive the settings it was derived from.
type limitsSnapshot struct {
	settings LimitsSettings
	gens     sync.Map // label -> uint64
}

type breakerSnapshot struct {
	settings BreakerSettings
	gens     sync.Map
}

type rateSnapshot struct {
	settings RateLimitSettings
	gens     sync.Map
}

func (s *limitsSnapshot) gen(label string) uint64 {
	return memoGen(&s.gens, label, func(l string) uint64 { return limitsGen(l, &s.settings) })
}

func (s *breakerSnapshot) gen(label string) uint64 {
	return memoGen(&s.gens, label, func(l string) uint64 { return breakerGen(l, &s.settings) })
}

func (s *rateSnapshot) gen(label string) uint64 {
	return memoGen(&s.gens, label, func(l string) uint64 { return rateGen(l, &s.settings) })
}

func memoGen(cache *sync.Map, label string, compute func(string) uint64) uint64 {
	if v, ok := cache.Load(label); ok {
		return v.(uint64)
	}
	g := compute(label)
	cache.Store(label, g)
	return g
}

// resolve is the limits half of [Manager.Resolve]: a real-action rule always
// beats the service marker; among real actions the most specific match wins.
func (s *limitsSnapshot) resolve(op chain.Operation) LimitsRef {
	p := s.settings.Default
	if i := s.best(op); i >= 0 {
		p = s.settings.Rules[i].Limits
	}
	return LimitsRef{Timeout: p.Timeout, Retry: p.Retry, Gen: s.gen(op.Service)}
}

func (s *limitsSnapshot) best(op chain.Operation) int {
	best := -1
	var bestSpec specificity
	for i := range s.settings.Rules {
		r := &s.settings.Rules[i]
		if r.Service != op.Service {
			continue
		}
		if isServiceScope(r.Action) {
			if best < 0 {
				best, bestSpec = i, specOf(r.Action, nil)
			}
			continue
		}
		if !r.Action.Matches(op.Request.Action) || !r.Match.Matches(op.Request.Metadata) {
			continue
		}
		sc := specOf(r.Action, r.Match)
		if best < 0 || sc.betterThan(bestSpec) {
			best, bestSpec = i, sc
		}
	}
	return best
}

// label is the label-level limits: the service-scope rule for the label, else
// the default. It is what the label-scoped retry budget is read from.
func (s *limitsSnapshot) label(label string) Limits {
	for i := range s.settings.Rules {
		r := &s.settings.Rules[i]
		if r.Service == label && isServiceScope(r.Action) {
			return r.Limits
		}
	}
	return s.settings.Default
}

// resolve is the breaker half of [Manager.Resolve]. A service-scope rule WINS
// OUTRIGHT: interface-level rules of that service are not consulted. Without a
// service rule, the most specific real-action rule applies.
func (s *breakerSnapshot) resolve(op chain.Operation) BreakerRef {
	if i := s.serviceRule(op.Service); i >= 0 {
		r := &s.settings.Rules[i]
		return BreakerRef{Policy: r.BreakerPolicy, Key: ruleKey(r.Service, i), Gen: s.gen(op.Service)}
	}
	if i := s.bestAction(op); i >= 0 {
		r := &s.settings.Rules[i]
		return BreakerRef{Policy: r.BreakerPolicy, Key: ruleKey(r.Service, i), Gen: s.gen(op.Service)}
	}
	return BreakerRef{Policy: s.settings.Default, Key: op.Service, Gen: s.gen(op.Service)}
}

func (s *breakerSnapshot) serviceRule(service string) int {
	for i := range s.settings.Rules {
		r := &s.settings.Rules[i]
		if r.Service == service && isServiceScope(r.Action) {
			return i
		}
	}
	return -1
}

func (s *breakerSnapshot) bestAction(op chain.Operation) int {
	best := -1
	var bestSpec specificity
	for i := range s.settings.Rules {
		r := &s.settings.Rules[i]
		if r.Service != op.Service || isServiceScope(r.Action) || !r.Action.Matches(op.Request.Action) {
			continue
		}
		sc := specOf(r.Action, nil)
		if best < 0 || sc.betterThan(bestSpec) {
			best, bestSpec = i, sc
		}
	}
	return best
}

func (s *breakerSnapshot) label(label string) BreakerPolicy {
	if i := s.serviceRule(label); i >= 0 {
		return s.settings.Rules[i].BreakerPolicy
	}
	return s.settings.Default
}

// resolve is the rate-limit half of [Manager.Resolve]. Rate limiting is
// interface-scoped only; the most specific matching real-action rule wins.
func (s *rateSnapshot) resolve(op chain.Operation) RateRef {
	p, key := s.settings.Default, op.Service
	best := -1
	var bestSpec specificity
	for i := range s.settings.Rules {
		r := &s.settings.Rules[i]
		if r.Service != op.Service || isServiceScope(r.Action) || !r.Action.Matches(op.Request.Action) {
			continue
		}
		sc := specOf(r.Action, nil)
		if best < 0 || sc.betterThan(bestSpec) {
			best, bestSpec, p, key = i, sc, r.RateLimitPolicy, ruleKey(r.Service, i)
		}
	}
	return RateRef{Policy: p, Key: key, Gen: s.gen(op.Service)}
}

func (s *rateSnapshot) label(string) RateLimitPolicy {
	return s.settings.Default
}

// limitsGen fingerprints the limits half of everything a label resolves: the
// default and every rule naming it, in order and with its position included —
// position matters because the state key embeds the rule's index.
func limitsGen(label string, s *LimitsSettings) uint64 {
	h := fnv.New64a()
	fingerprint(h, "default")
	fingerprintLimits(h, s.Default)
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service != label {
			continue
		}
		fingerprintRuleHead(h, i, r.Service, r.Action, r.Match)
		fingerprintLimits(h, r.Limits)
	}
	return h.Sum64()
}

// breakerGen fingerprints the breaker half of a label's rules.
func breakerGen(label string, s *BreakerSettings) uint64 {
	h := fnv.New64a()
	fingerprint(h, "default")
	fingerprintBreaker(h, s.Default)
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service != label {
			continue
		}
		fingerprintRuleHead(h, i, r.Service, r.Action, nil)
		fingerprintBreaker(h, r.BreakerPolicy)
	}
	return h.Sum64()
}

// rateGen fingerprints the rate-limit half of a label's rules.
func rateGen(label string, s *RateLimitSettings) uint64 {
	h := fnv.New64a()
	fingerprint(h, "default")
	fingerprintRate(h, s.Default)
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service != label {
			continue
		}
		fingerprintRuleHead(h, i, r.Service, r.Action, nil)
		fingerprintRate(h, r.RateLimitPolicy)
	}
	return h.Sum64()
}

// fingerprintRuleHead writes the parts every capability's fingerprint shares:
// the rule's position and selector. A selector change moves all three
// generations, because it changes which calls a rule's state belongs to.
func fingerprintRuleHead(h hash.Hash64, index int, service string, action chain.ActionMatch, match matcher.ExactMap) {
	fingerprint(h, "rule")
	fingerprint(h, strconv.Itoa(index))
	fingerprint(h, service)
	fingerprint(h, action.Exact)
	fingerprint(h, action.Prefix)
	fingerprintSelector(h, match)
}

// fingerprintLimits writes the limits half (timeout and retry schedule).
func fingerprintLimits(h hash.Hash64, l Limits) {
	fingerprint(h, strconv.FormatInt(int64(l.Timeout), 10))
	fingerprint(h, fmt.Sprintf("%+v", l.Retry))
}

// breakerFingerprint is the breaker half, in one value so it can be
// fingerprinted without pulling in unrelated knobs.
type breakerFingerprint struct {
	Off         bool
	Threshold   int
	Open        time.Duration
	Strategy    circuitbreaker.Strategy
	Rate        float64
	MinRequests int
	Window      time.Duration
	Slow        time.Duration
	SlowRate    float64
	HalfOpen    int
	HalfTimeout time.Duration
}

func fingerprintBreaker(h hash.Hash64, p BreakerPolicy) {
	fingerprint(h, fmt.Sprintf("%+v", breakerFingerprint{
		Off:         p.BreakerDisabled,
		Threshold:   p.ErrorThreshold,
		Open:        p.OpenDuration,
		Strategy:    p.BreakerStrategy,
		Rate:        p.ErrorRateThreshold,
		MinRequests: p.MinRequests,
		Window:      p.BreakerWindow,
		Slow:        p.SlowCallDurationThreshold,
		SlowRate:    p.SlowCallRateThreshold,
		HalfOpen:    p.HalfOpenRequests,
		HalfTimeout: p.HalfOpenTimeout,
	}))
}

// rateFingerprint is the rate-limit half.
type rateFingerprint struct {
	Rate       float64
	Burst      int
	MaxWait    time.Duration
	Algorithm  ratelimit.Algorithm
	Window     time.Duration
	WarmUp     time.Duration
	ColdFactor int
}

func fingerprintRate(h hash.Hash64, p RateLimitPolicy) {
	fingerprint(h, fmt.Sprintf("%+v", rateFingerprint{
		Rate:       p.RateLimit,
		Burst:      p.Burst,
		MaxWait:    p.RateLimitMaxWait,
		Algorithm:  p.Algorithm,
		Window:     p.Window,
		WarmUp:     p.RateLimitWarmUpDuration,
		ColdFactor: p.WarmUpColdFactor,
	}))
}

// mixGens folds the three capability generations into one for backends that
// reload every family together and therefore cannot use them separately.
func mixGens(a, b, c uint64) uint64 {
	h := fnv.New64a()
	fingerprint(h, strconv.FormatUint(a, 10))
	fingerprint(h, strconv.FormatUint(b, 10))
	fingerprint(h, strconv.FormatUint(c, 10))
	return h.Sum64()
}

// fingerprint writes one field, delimited so that ("ab","c") and ("a","bc")
// cannot collide.
func fingerprint(h hash.Hash64, s string) {
	h.Write([]byte(s))
	h.Write([]byte{0})
}

// fingerprintSelector writes a match selector in key order — a Go map iterates
// nondeterministically, and a generation that changed on every process start
// would rebuild every breaker for no reason.
func fingerprintSelector(h hash.Hash64, sel matcher.ExactMap) {
	if len(sel) == 0 {
		h.Write([]byte{0})
		return
	}
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fingerprint(h, k)
		fingerprint(h, sel[k])
	}
}
