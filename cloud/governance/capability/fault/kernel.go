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
	"context"
	"sync/atomic"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/timeutil"
)

// kernel.go is the SHARED injection sequence and the guardrail state model. A
// direction package owns its own [Kernel] instance (instantiated with its own
// configuration type) and its own rule type; the sequence — ordered selection,
// guardrails, latency, effect — is written once here so the two directions can
// never drift apart.

// Rule is the mechanism seam between the shared kernel and a direction's rule
// type: the kernel matches, draws and guards through its [RuleCore], and the
// direction supplies its own effect through [Roll].
type Rule interface {
	// Core exposes the direction-neutral targeting and dose knobs.
	Core() *RuleCore

	// Roll attaches the direction's effect to the dose the kernel decided. The
	// returned [Result] carries the rule's name and the latency; when the dose
	// injects it also carries the direction's effect fields and a non-nil error
	// for the caller to return instead of running the real operation.
	Roll(Dose) (Result, error)
}

// Kernel is one traffic direction's injection kernel: the stable shell a push
// swaps by publishing a new [generation].
//
// C is the direction's configuration type, so [Kernel.Config] hands back the
// document exactly as pushed without a shared config shape. The zero value is
// an unarmed kernel: it decides nothing until [Kernel.Refresh] is called.
//
// The call path loads the published generation once per attempt, so a push
// landing mid-call cannot mix two documents. Refresh is UNSYNCHRONIZED: pushes
// arrive from one serialized source loop, so seriality is the control plane's
// contract, not this type's lock.
type Kernel[C any] struct {
	cur atomic.Pointer[generation[C]]
}

// generation is one published kernel: the document as pushed, whose rule order
// IS precedence, plus the per-rule guardrail budgets reached by name. It is
// built on every push and never mutated afterwards except through the budgets'
// atomics, so a caller that loaded it can use it for the attempt's whole
// duration without racing a push.
type generation[C any] struct {
	cfg     C
	runID   string
	rules   []Rule
	budgets *Budgets
}

// NewKernel returns an unarmed kernel for one direction's configuration type.
func NewKernel[C any]() *Kernel[C] {
	return &Kernel[C]{}
}

// Refresh publishes a new generation built from cfg, runID and rules. It
// RECONCILES the guardrail budgets against the previous generation by rule
// NAME, but only when the run id is unchanged: a rule with the same name reuses
// its counters, so a push neither resets a live rule nor re-arms one that
// already expired. A new run id starts every rule from zero — the platform
// re-arms the same rule names for every run, and a spent MaxAffected from the
// previous drill must not silently disable the next one.
//
// cfg is ALREADY validated: every expr tag runs where the document enters.
// Callers that construct a configuration in code must validate it themselves.
func (k *Kernel[C]) Refresh(cfg C, runID string, rules []Rule) {
	var prev *generation[C]
	if p := k.cur.Load(); p != nil {
		prev = p
	}
	var prevRunID string
	var prevBudgets *Budgets
	if prev != nil {
		prevRunID, prevBudgets = prev.runID, prev.budgets
	}
	k.cur.Store(&generation[C]{
		cfg:     cfg,
		runID:   runID,
		rules:   rules,
		budgets: reconcileBudgets(runID, prevRunID, prevBudgets, rules),
	})
}

// Config returns the document this kernel currently runs — the cfg inside the
// published generation, so there is exactly one copy of it (the zero value when
// nothing has been pushed, which is a no-op).
func (k *Kernel[C]) Config() C {
	if g := k.cur.Load(); g != nil {
		return g.cfg
	}
	var zero C
	return zero
}

// Fire runs one attempt's roll-and-delay sequence against the generation current
// when it is called, and reports what it decided.
//
// It does NOT call the operation: the returned error is the whole "may I
// proceed?" signal — nil means the caller should run the real fn, anything else
// means it should return that error instead (an injected failure, or the
// context error when the injected latency was cancelled mid-sleep).
//
// Selection is an ORDERED list: the FIRST matching rule governs the attempt and
// evaluation stops there. A matched rule that does not fire (Rate draw missed,
// guardrail tripped, blast radius full) still terminates selection and leaves
// the attempt untouched; there is no fall-through to the next matching rule.
//
// The latency sleep is cancellable via ctx; on cancel it returns the context
// error (with the decision intact) so the caller's own timeout/budget logic
// reacts rather than retrying blindly. Fire does NOT retry — retry is a client
// concern.
func (k *Kernel[C]) Fire(ctx context.Context, t Target) (Result, error) {
	g := k.cur.Load()
	if g == nil { // nothing pushed yet: this direction is a transparent no-op
		return Result{}, nil
	}
	for _, r := range g.rules {
		core := r.Core()
		if !core.Matches(t) {
			continue
		}
		b := g.budgets.get(core.Name)
		// The governing rule's own guardrails: a forgotten fire self-heals instead
		// of running until the next deploy.
		if b.durationExhausted(core.MaxDuration) {
			b.reportExhaustion(ctx, core.Name, "max-duration")
			return Result{}, nil
		}
		dose := core.Dose(b, time.Now())
		if !dose.Inject && dose.Delay == 0 {
			return Result{}, nil // the draw missed, or the rule carries no effect
		}
		o, err := r.Roll(dose)
		// Spend one unit of the blast-radius budget only after the rule actually
		// drew a dose, so the cap is exact even under concurrency.
		if !b.spend(core.MaxAffected) {
			b.reportExhaustion(ctx, core.Name, "max-affected")
			return Result{}, nil
		}
		o.RunID = g.runID
		if o.Delay > 0 && !timeutil.Sleep(ctx, o.Delay) {
			return o, ctx.Err()
		}
		return o, err
	}
	return Result{}, nil
}

// Budgets is the guardrail table one generation carries: one [Budget] per rule
// name. It is the only mutable state behind a push.
type Budgets struct {
	byName map[string]*Budget
}

// reconcileBudgets builds the guardrail table for rules, reusing prev's entries
// by name only when the run id is unchanged (see [Kernel.Refresh]).
func reconcileBudgets(runID, prevRunID string, prev *Budgets, rules []Rule) *Budgets {
	b := &Budgets{byName: make(map[string]*Budget, len(rules))}
	for _, r := range rules {
		name := r.Core().Name
		var budget *Budget
		if prev != nil && runID == prevRunID {
			budget = prev.byName[name]
		}
		if budget == nil {
			budget = &Budget{}
		}
		b.byName[name] = budget
	}
	return b
}

// get returns the budget for a rule name. Every name in the table came from the
// same rule list the kernel scans, so a hit is guaranteed.
func (bs *Budgets) get(name string) *Budget { return bs.byName[name] }

// Budget is one rule identity's guardrail state across pushes: when its first
// affected call happened (the MaxDuration window), and how many calls it has
// touched (the MaxAffected budget). It is shared by every generation the
// identity has had, so a push neither resets a rule's budget nor re-arms one
// that already expired; dropping the identity drops its last reference.
type Budget struct {
	firstAt   atomic.Int64 // unix-nanos of the rule's first affected call; 0 = none
	affected  atomic.Int64 // calls this rule actually touched
	notified  atomic.Bool  // the exhaustion transition was reported
	matched   atomic.Int64 // matching calls seen (the Every counter)
	firstSeen atomic.Int64 // unix-nanos of the first MATCHING call; 0 = none
}

// noteMatch records a matching call and returns its 1-based sequence number.
// The first match also starts the rule's StartAfter/On-Off clock, which is why
// the window is per rule identity rather than per push.
func (b *Budget) noteMatch(now time.Time) uint64 {
	b.firstSeen.CompareAndSwap(0, now.UnixNano())
	return uint64(b.matched.Add(1))
}

// inWindow reports whether the rule is currently in an eligible phase. The
// clock starts at the rule's first matching call plus StartAfter; with no
// On/Off pair it is always eligible after that.
func (b *Budget) inWindow(r RuleCore, now time.Time) bool {
	first := b.firstSeen.Load()
	if first == 0 {
		return false
	}
	start := time.Unix(0, first).Add(r.StartAfter)
	if now.Before(start) {
		return false
	}
	if r.On > 0 && r.Off > 0 {
		phase := now.Sub(start) % (r.On + r.Off)
		return phase < r.On
	}
	return true
}

// reportExhaustion logs the ONE transition into exhaustion for this rule
// identity, so an operator sees why a fire stopped without polling per-process
// counters. Repeated no-op calls after the trip stay silent.
func (b *Budget) reportExhaustion(ctx context.Context, ruleName, reason string) {
	if b.notified.CompareAndSwap(false, true) {
		log.Info(ctx, log.TagAppDef, "fault: rule exhausted; it stops firing for this run",
			log.String("rule", ruleName), log.String("reason", reason))
	}
}

// durationExhausted reports whether the MaxDuration window has elapsed. It is
// the time half of the guardrails, checked on the hot path before the draw (the
// count half is enforced by [Budget.spend] when a roll actually fires).
func (b *Budget) durationExhausted(max time.Duration) bool {
	if max <= 0 {
		return false
	}
	first := b.firstAt.Load()
	return first != 0 && time.Since(time.Unix(0, first)) > max
}

// spend books one affected call against the MaxAffected budget, or reports
// that the budget is already spent. It is called AFTER a rule rolled a dose and
// only then, so the budget is an exact cap: a concurrent batch of in-flight
// attempts cannot overshoot it (the CAS loop admits at most MaxAffected
// winners), and an attempt whose rate did not hit never spends one. The first
// winner also starts the MaxDuration window; the CAS makes that race-safe.
func (b *Budget) spend(maxAffected int64) bool {
	if maxAffected > 0 {
		for {
			n := b.affected.Load()
			if n >= maxAffected {
				return false
			}
			if b.affected.CompareAndSwap(n, n+1) {
				b.firstAt.CompareAndSwap(0, time.Now().UnixNano())
				return true
			}
		}
	}
	b.affected.Add(1)
	b.firstAt.CompareAndSwap(0, time.Now().UnixNano())
	return true
}
