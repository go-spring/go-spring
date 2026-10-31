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
	"errors"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// TestRefreshConcurrentWithFire pins that a published kernel is immutable: a
// refresh allocates fresh slots and shares only the guardrail budget, so a fire
// that loaded the old kernel can read it while a push lands. Run with -race —
// mutating a reused slot's rule in place is what this test catches.
func TestRefreshConcurrentWithFire(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{{Name: "fire", Service: "svc", Rate: 0.5, Class: ClassTransient, Error: ErrorGeneric}}})

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				m.Refresh(Config{Rules: []Rule{{Name: "fire", Service: "svc", Rate: 0.5, Class: ClassTransient, Error: ErrorTimeout}}})
			}
		}
	}()
	for i := 0; i < 50000; i++ {
		_, _ = m.Fire(context.Background(), Target{Service: "svc"})
	}
	close(stop)
	wg.Wait()
}

// armed builds a one-rule document that always fires on service, bounded by
// maxAffected (0 = unbounded).
func armed(name, service string, maxAffected int64) Config {
	return Config{Rules: []Rule{{
		Name: name, Service: service, Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: maxAffected,
	}}}
}

// TestRefresh_NameIdentitySurvivesReorder pins the reason identity is a name:
// position carries precedence, never identity. Reordering the list must not
// reset a spent guardrail.
func TestRefresh_NameIdentitySurvivesReorder(t *testing.T) {
	m := NewManager()
	cfg := Config{Rules: []Rule{
		{Name: "a", Service: "svc-a", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: 1},
		{Name: "b", Service: "svc-b", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: 1},
	}}
	m.Refresh(cfg)

	// Spend a's budget.
	_, err := m.Fire(context.Background(), Target{Service: "svc-a"})
	assert.Error(t, err).NotNil()

	// Reorder the same two rules.
	m.Refresh(Config{Rules: []Rule{cfg.Rules[1], cfg.Rules[0]}})

	// a is still spent: the reorder did not reset its budget, so it does not
	// fire and — because only fired rules are reported — the decision is empty.
	d, err := m.Fire(context.Background(), Target{Service: "svc-a"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
	assert.That(t, d.Fired()).False()

	// b never fired before, so its budget is intact.
	_, err = m.Fire(context.Background(), Target{Service: "svc-b"})
	assert.That(t, IsInjected(err)).True()
}

// TestRefresh_NameChangeResetsBudget pins the other half of the contract: a
// rename IS a new experiment, so it starts with a fresh budget.
func TestRefresh_NameChangeResetsBudget(t *testing.T) {
	m := NewManager()
	m.Refresh(armed("old", "svc", 1))
	_, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	_, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // spent

	m.Refresh(armed("new", "svc", 1))
	_, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True() // a fresh experiment
}

// TestDecision_RuleIsConfiguredName: the rule a call reports is the configured
// identity — that is the key a platform joins telemetry on.
func TestResult_NamesTheGoverningRule(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{
		{Name: "any-svc", Service: "anything", Rate: 1, Class: ClassTransient, Error: ErrorGeneric},
	}})
	d, err := m.Fire(context.Background(), Target{Service: "anything"})
	assert.Error(t, err).NotNil()
	assert.That(t, d.Fired()).True()
	assert.String(t, d.Name).Equal("any-svc")
}

// TestGuardrails_TrackAffectedAndExhaustion pins the internal guardrail state:
// the affected counter, the first-affected clock and the tripped bound.
func TestGuardrails_TrackAffectedAndExhaustion(t *testing.T) {
	m := NewManager()
	assert.That(t, m.core.Load()).Nil() // nothing pushed

	m.Refresh(Config{Rules: []Rule{
		{Name: "spent", Service: "svc-a", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: 1},
		{Name: "free", Service: "svc-b", Rate: 1, Class: ClassTransient, Error: ErrorGeneric},
	}})

	_, _ = m.Fire(context.Background(), Target{Service: "svc-a"})
	_, _ = m.Fire(context.Background(), Target{Service: "svc-a"}) // already spent

	k := m.core.Load()
	assert.Number(t, len(k.budgets)).Equal(2)

	spentRule, freeRule := k.cfg.Rules[0], k.cfg.Rules[1]
	assert.String(t, spentRule.Name).Equal("spent")
	assert.Number(t, int(k.budgets["spent"].affected.Load())).Equal(1)
	assert.That(t, k.budgets["spent"].affected.Load() >= spentRule.MaxAffected).True()
	assert.That(t, k.budgets["spent"].firstAt.Load() != 0).True()

	assert.String(t, freeRule.Name).Equal("free")
	assert.Number(t, int(k.budgets["free"].affected.Load())).Equal(0)
	assert.That(t, k.budgets["free"].durationExhausted(freeRule)).False()
	assert.Number(t, k.budgets["free"].firstAt.Load()).Equal(0)
}

func TestGuardrails_MaxDurationExhausts(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{{
		Name: "timed", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxDuration: 10 * time.Millisecond,
	}}})
	_, _ = m.Fire(context.Background(), Target{Service: "svc"})
	time.Sleep(25 * time.Millisecond)

	k := m.core.Load()
	assert.Number(t, len(k.budgets)).Equal(1)
	assert.That(t, k.budgets["timed"].durationExhausted(k.cfg.Rules[0])).True()
}

// TestEvaluate_FirstMatchWins pins the selection model: rules are an ordered
// list and the FIRST matching rule governs the attempt. A later rule that also
// matches never runs, so a call is never affected by two experiments.
func TestEvaluate_FirstMatchWins(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{
		{Name: "slow", Service: "svc", Latency: 2 * time.Millisecond},
		{Name: "broken", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorReset},
	}})
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // the second rule never ran
	assert.That(t, d.Fired()).True()
	assert.String(t, d.Name).Equal("slow")
	assert.That(t, d.Delay >= 2*time.Millisecond).True()
}

// TestEvaluate_OrderIsPrecedence pins what ordering means: reordering the
// document changes which rule governs the same call.
func TestEvaluate_OrderIsPrecedence(t *testing.T) {
	slow := Rule{Name: "slow", Service: "svc", Latency: time.Millisecond}
	broken := Rule{Name: "broken", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorTimeout}

	m := NewManager()
	m.Refresh(Config{Rules: []Rule{slow, broken}})
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.String(t, d.Name).Equal("slow")

	m.Refresh(Config{Rules: []Rule{broken, slow}})
	d, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, errors.Is(err, context.DeadlineExceeded)).True()
	assert.String(t, d.Name).Equal("broken")
}

// TestEvaluate_DisarmedFirstMatchStillGoverns pins that a DISARMED rule is
// still a match: listed first it governs the attempt (and draws nothing), so a
// later armed rule on the same service never reaches it. Order is precedence.
func TestEvaluate_DisarmedFirstMatchStillGoverns(t *testing.T) {
	armed := Rule{Name: "special", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorReset}
	disarmed := Rule{Name: "disarmed", Service: "svc", Rate: 0, Class: ClassTransient, Error: ErrorGeneric} // matches, never fires

	m := NewManager()
	m.Refresh(Config{Rules: []Rule{disarmed, armed}})
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // the disarmed rule governed and drew nothing
	assert.That(t, d.Fired()).False()

	m.Refresh(Config{Rules: []Rule{armed, disarmed}})
	d, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
	assert.String(t, d.Name).Equal("special")
}

// TestEvaluate_MatchedRuleMissDoesNotFallThrough: matching decides governance
// and the Rate draw decides whether it fires; a miss does NOT hand the attempt
// to the next matching rule.
func TestEvaluate_MatchedRuleMissDoesNotFallThrough(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{
		{Name: "never", Service: "svc", Rate: 0, Class: ClassTransient, Error: ErrorGeneric}, // matches, never fires
		{Name: "second", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorReset},  // would have fired
	}})
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
}

// TestSpend_MaxAffectedExactUnderConcurrency pins that the blast-radius cap is
// exact: concurrent in-flight attempts cannot overshoot it, because a rule
// spends one budget unit before the dose is applied.
func TestSpend_MaxAffectedExactUnderConcurrency(t *testing.T) {
	const budget = 10
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{
		{Name: "fire", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: budget},
	}})
	var wg sync.WaitGroup
	var injected atomic.Int64
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := m.Fire(context.Background(), Target{Service: "svc"}); IsInjected(err) {
				injected.Add(1)
			}
		}()
	}
	wg.Wait()
	assert.Number(t, int(injected.Load())).Equal(budget)
	assert.Number(t, int(m.core.Load().budgets["fire"].affected.Load())).Equal(budget)
}

// TestEvaluate_CarriesRunID pins the drill label: the decision carries the
// document's run id, so spans and records from one drill group.
func TestEvaluate_CarriesRunID(t *testing.T) {
	m := NewManager()
	m.Refresh(Config{RunID: "drill-1", Rules: []Rule{{Name: "fire", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorGeneric}}})
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	assert.String(t, d.RunID).Equal("drill-1")
	assert.String(t, m.Config().RunID).Equal("drill-1")
	assert.Number(t, int(m.core.Load().budgets["fire"].affected.Load())).Equal(1)
}

// TestRefresh_SameRunRepushKeepsBudget pins the idempotent half of the identity:
// a controller re-pushing the same document within one run (its tick) must not
// re-arm a guardrail that already tripped.
func TestRefresh_SameRunRepushKeepsBudget(t *testing.T) {
	cfg := Config{RunID: "drill-1", Rules: []Rule{{
		Name: "fire", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: 1,
	}}}
	m := NewManager()
	m.Refresh(cfg)
	_, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()

	// Same document, same run: the spent budget survives.
	m.Refresh(cfg)
	_, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.String(t, m.Config().RunID).Equal("drill-1")
}

// TestRefresh_NewRunResetsBudget pins the other half: a new run id is a new
// execution, so a rule whose budget the previous drill spent fires again. Without
// this, every drill after the first would silently do nothing.
func TestRefresh_NewRunResetsBudget(t *testing.T) {
	rule := Rule{Name: "fire", Service: "svc", Rate: 1, Class: ClassTransient, Error: ErrorGeneric, MaxAffected: 1}
	m := NewManager()
	m.Refresh(Config{RunID: "drill-1", Rules: []Rule{rule}})
	_, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	_, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // spent in drill-1

	// Same rule, new run: a fresh budget.
	m.Refresh(Config{RunID: "drill-2", Rules: []Rule{rule}})
	_, err = m.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	assert.String(t, m.Config().RunID).Equal("drill-2")
}

// TestRefresh_StopDoesNotCancelInFlight pins the meaning of "stop": it stops
// producing NEW fires, it does not unwind an attempt already affected. The
// in-flight sleep completes and returns its decision; the next attempt matches
// nothing.
func TestRefresh_StopDoesNotCancelInFlight(t *testing.T) {
	const latency = 200 * time.Millisecond
	m := NewManager()
	m.Refresh(Config{Rules: []Rule{{
		Name: "slow", Service: "svc", Latency: latency,
	}}})

	type result struct {
		d   Result
		err error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		d, err := m.Fire(context.Background(), Target{Service: "svc"})
		done <- result{d: d, err: err}
	}()

	// Wait until the attempt has claimed the rule — it is now inside the sleep.
	deadline := time.Now().Add(time.Second)
	for m.core.Load().budgets["slow"].affected.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the fire never started")
		}
		time.Sleep(time.Millisecond)
	}

	// Stop: remove the rule from the armed document.
	m.Refresh(Config{})

	r := <-done
	assert.Error(t, r.err).Nil()
	assert.That(t, r.d.Fired()).True()
	assert.That(t, time.Since(start) >= latency).True() // the sleep was not cut short

	// A call arriving after the stop is not fired at all.
	start = time.Now()
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
	assert.That(t, d.Fired()).False()
	assert.That(t, time.Since(start) < latency).True()
}

// TestManager_UnarmedIsTransparent pins the unarmed authority's contract: no
// document pushed means no rules, so Fire decides nothing (zero Result, nil
// error — the caller runs the real fn) and Config reports the zero document.
func TestManager_UnarmedIsTransparent(t *testing.T) {
	m := NewManager()
	d, err := m.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
	assert.That(t, d == Result{}).True()
	assert.That(t, len(m.Config().Rules)).Equal(0)
}

// TestResult_Fired pins what counts as "actually affected": an injected error,
// an applied latency — including one whose sleep was cancelled (the fire was
// set) — and nothing on the zero value.
func TestResult_Fired(t *testing.T) {
	assert.That(t, Result{}.Fired()).False()
	assert.That(t, Result{Name: "r"}.Fired()).False() // governed but the roll missed
	assert.That(t, Result{Inject: true}.Fired()).True()
	assert.That(t, Result{Delay: time.Millisecond}.Fired()).True() // cancelled latency still counts
	assert.That(t, Result{Inject: true, Delay: time.Millisecond}.Fired()).True()
}
