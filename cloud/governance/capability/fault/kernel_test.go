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
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

// testConfig stands for a direction's configuration type: the kernel must hand
// back exactly what it published without knowing its shape.
type testConfig struct{ RunID string }

// testRule stands for a direction's rule type: it adds the outbound-style effect
// on top of the shared core, exactly as the direction packages do.
type testRule struct {
	RuleCore
	Error string
	Class string
}

func (r testRule) Core() *RuleCore { return &r.RuleCore }

func (r testRule) Roll(d Dose) (Result, error) {
	o := Result{Name: r.Name, Delay: d.Delay}
	if !d.Inject {
		return o, nil
	}
	o.Inject = true
	o.Error = r.Error
	o.Class = r.Class
	return o, NewInjectedError(r.Error, r.Class, 0)
}

func rules(rs ...testRule) []Rule {
	out := make([]Rule, len(rs))
	for i := range rs {
		out[i] = rs[i]
	}
	return out
}

// armed builds a one-rule kernel that always fires on service, bounded by
// maxAffected (0 = unbounded).
func armed(t *testing.T, name, service string, maxAffected int64) *Kernel[testConfig] {
	t.Helper()
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: name, Service: service, Rate: 1, MaxAffected: maxAffected},
		Error:    ErrorGeneric,
		Class:    ClassTransient,
	}))
	return k
}

// TestRefreshConcurrentWithFire pins that a published kernel is immutable: a
// refresh allocates fresh slots and shares only the guardrail budget, so a fire
// that loaded the old kernel can read it while a push lands. Run with -race —
// mutating a reused slot's rule in place is what this test catches.
func TestRefreshConcurrentWithFire(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 0.5},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
				k.Refresh(testConfig{}, "", rules(testRule{
					RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 0.5},
					Error:    ErrorTimeout, Class: ClassTransient,
				}))
			}
		}
	})
	for range 50000 {
		_, _ = k.Fire(context.Background(), Target{Service: "svc"})
	}
	close(stop)
	wg.Wait()
}

// TestRefresh_NameIdentitySurvivesReorder pins the reason identity is a name:
// position carries precedence, never identity. Reordering the list must not
// reset a spent guardrail.
func TestRefresh_NameIdentitySurvivesReorder(t *testing.T) {
	a := testRule{RuleCore: RuleCore{Name: "a", Service: "svc-a", Rate: 1, MaxAffected: 1}, Error: ErrorGeneric, Class: ClassTransient}
	b := testRule{RuleCore: RuleCore{Name: "b", Service: "svc-b", Rate: 1, MaxAffected: 1}, Error: ErrorGeneric, Class: ClassTransient}

	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(a, b))

	// Spend a's budget.
	_, err := k.Fire(context.Background(), Target{Service: "svc-a"})
	assert.Error(t, err).NotNil()

	// Reorder the same two rules.
	k.Refresh(testConfig{}, "", rules(b, a))

	// a is still spent: the reorder did not reset its budget, so it does not
	// fire and — because only fired rules are reported — the decision is empty.
	d, err := k.Fire(context.Background(), Target{Service: "svc-a"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()

	// b never fired before, so its budget is intact.
	_, err = k.Fire(context.Background(), Target{Service: "svc-b"})
	assert.That(t, IsInjected(err)).True()
}

// TestRefresh_NameChangeResetsBudget pins the other half of the contract: a
// rename IS a new experiment, so it starts with a fresh budget.
func TestRefresh_NameChangeResetsBudget(t *testing.T) {
	k := armed(t, "old", "svc", 1)
	_, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	_, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // spent

	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "new", Service: "svc", Rate: 1, MaxAffected: 1},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))
	_, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True() // a fresh experiment
}

// TestResult_NamesTheGoverningRule: the rule a call reports is the configured
// identity — that is the key a platform joins telemetry on.
func TestResult_NamesTheGoverningRule(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "any-svc", Service: "anything", Rate: 1},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))
	d, err := k.Fire(context.Background(), Target{Service: "anything"})
	assert.Error(t, err).NotNil()
	assert.That(t, d.Fired()).True()
	assert.String(t, d.Name).Equal("any-svc")
}

// TestGuardrails_TrackAffectedAndExhaustion pins the internal guardrail state:
// the affected counter, the first-affected clock and the tripped bound.
func TestGuardrails_TrackAffectedAndExhaustion(t *testing.T) {
	k := NewKernel[testConfig]()
	assert.That(t, k.cur.Load()).Nil() // nothing pushed

	k.Refresh(testConfig{}, "", rules(
		testRule{RuleCore: RuleCore{Name: "spent", Service: "svc-a", Rate: 1, MaxAffected: 1}, Error: ErrorGeneric, Class: ClassTransient},
		testRule{RuleCore: RuleCore{Name: "free", Service: "svc-b", Rate: 1}, Error: ErrorGeneric, Class: ClassTransient},
	))

	_, _ = k.Fire(context.Background(), Target{Service: "svc-a"})
	_, _ = k.Fire(context.Background(), Target{Service: "svc-a"}) // already spent

	g := k.cur.Load()
	assert.Number(t, len(g.budgets.byName)).Equal(2)
	assert.Number(t, int(g.budgets.byName["spent"].affected.Load())).Equal(1)
	assert.That(t, g.budgets.byName["spent"].affected.Load() >= 1).True()
	assert.That(t, g.budgets.byName["spent"].firstAt.Load() != 0).True()

	assert.Number(t, int(g.budgets.byName["free"].affected.Load())).Equal(0)
	assert.That(t, g.budgets.byName["free"].durationExhausted(0)).False()
	assert.Number(t, g.budgets.byName["free"].firstAt.Load()).Equal(0)
}

func TestGuardrails_MaxDurationExhausts(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "timed", Service: "svc", Rate: 1, MaxDuration: 10 * time.Millisecond},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))
	_, _ = k.Fire(context.Background(), Target{Service: "svc"})
	time.Sleep(25 * time.Millisecond)

	g := k.cur.Load()
	assert.Number(t, len(g.budgets.byName)).Equal(1)
	assert.That(t, g.budgets.byName["timed"].durationExhausted(10*time.Millisecond)).True()
}

// TestEvaluate_FirstMatchWins pins the selection model: rules are an ordered
// list and the FIRST matching rule governs the attempt. A later rule that also
// matches never runs, so a call is never affected by two experiments.
func TestEvaluate_FirstMatchWins(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(
		testRule{RuleCore: RuleCore{Name: "slow", Service: "svc", Latency: 2 * time.Millisecond}},
		testRule{RuleCore: RuleCore{Name: "broken", Service: "svc", Rate: 1}, Error: ErrorReset, Class: ClassTransient},
	))
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // the second rule never ran
	assert.That(t, d.Fired()).True()
	assert.String(t, d.Name).Equal("slow")
	assert.That(t, d.Delay >= 2*time.Millisecond).True()
}

// TestEvaluate_OrderIsPrecedence pins what ordering means: reordering the
// document changes which rule governs the same call.
func TestEvaluate_OrderIsPrecedence(t *testing.T) {
	slow := testRule{RuleCore: RuleCore{Name: "slow", Service: "svc", Latency: time.Millisecond}}
	broken := testRule{RuleCore: RuleCore{Name: "broken", Service: "svc", Rate: 1}, Error: ErrorTimeout, Class: ClassTransient}

	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(slow, broken))
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.String(t, d.Name).Equal("slow")

	k.Refresh(testConfig{}, "", rules(broken, slow))
	d, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, errors.Is(err, os.ErrDeadlineExceeded)).True()
	assert.String(t, d.Name).Equal("broken")
}

// TestEvaluate_DisarmedFirstMatchStillGoverns pins that a DISARMED rule is
// still a match: listed first it governs the attempt (and draws nothing), so a
// later armed rule on the same service never reaches it. Order is precedence.
func TestEvaluate_DisarmedFirstMatchStillGoverns(t *testing.T) {
	armedRule := testRule{RuleCore: RuleCore{Name: "special", Service: "svc", Rate: 1}, Error: ErrorReset, Class: ClassTransient}
	disarmed := testRule{RuleCore: RuleCore{Name: "disarmed", Service: "svc", Rate: 0}, Error: ErrorGeneric, Class: ClassTransient}

	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(disarmed, armedRule))
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // the disarmed rule governed and drew nothing
	assert.That(t, d.Fired()).False()

	k.Refresh(testConfig{}, "", rules(armedRule, disarmed))
	d, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, errors.Is(err, syscall.ECONNRESET)).True()
	assert.String(t, d.Name).Equal("special")
}

// TestEvaluate_MatchedRuleMissDoesNotFallThrough: matching decides governance
// and the Rate draw decides whether it fires; a miss does NOT hand the attempt
// to the next matching rule.
func TestEvaluate_MatchedRuleMissDoesNotFallThrough(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(
		testRule{RuleCore: RuleCore{Name: "never", Service: "svc", Rate: 0}, Error: ErrorGeneric, Class: ClassTransient},
		testRule{RuleCore: RuleCore{Name: "second", Service: "svc", Rate: 1}, Error: ErrorReset, Class: ClassTransient},
	))
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
}

// TestSpend_MaxAffectedExactUnderConcurrency pins that the blast-radius cap is
// exact: concurrent in-flight attempts cannot overshoot it, because a rule
// spends one budget unit before the dose is applied.
func TestSpend_MaxAffectedExactUnderConcurrency(t *testing.T) {
	const budget = 10
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 1, MaxAffected: budget},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))
	var wg sync.WaitGroup
	var injected atomic.Int64
	for range 50 {
		wg.Go(func() {
			if _, err := k.Fire(context.Background(), Target{Service: "svc"}); IsInjected(err) {
				injected.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Number(t, int(injected.Load())).Equal(budget)
	assert.Number(t, int(k.cur.Load().budgets.byName["fire"].affected.Load())).Equal(budget)
}

// TestEvaluate_CarriesRunID pins the drill label: the decision carries the
// document's run id, so spans and records from one drill group.
func TestEvaluate_CarriesRunID(t *testing.T) {
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{RunID: "drill-1"}, "drill-1", rules(testRule{
		RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 1},
		Error:    ErrorGeneric, Class: ClassTransient,
	}))
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	assert.String(t, d.RunID).Equal("drill-1")
	assert.String(t, k.Config().RunID).Equal("drill-1")
	assert.Number(t, int(k.cur.Load().budgets.byName["fire"].affected.Load())).Equal(1)
}

// TestRefresh_SameRunRepushKeepsBudget pins the idempotent half of the identity:
// a controller re-pushing the same document within one run (its tick) must not
// re-arm a guardrail that already tripped.
func TestRefresh_SameRunRepushKeepsBudget(t *testing.T) {
	cfg := testConfig{RunID: "drill-1"}
	rs := rules(testRule{
		RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 1, MaxAffected: 1},
		Error:    ErrorGeneric, Class: ClassTransient,
	})
	k := NewKernel[testConfig]()
	k.Refresh(cfg, "drill-1", rs)
	_, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()

	// Same document, same run: the spent budget survives.
	k.Refresh(cfg, "drill-1", rs)
	_, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.String(t, k.Config().RunID).Equal("drill-1")
}

// TestRefresh_NewRunResetsBudget pins the other half: a new run id is a new
// execution, so a rule whose budget the previous drill spent fires again. Without
// this, every drill after the first would silently do nothing.
func TestRefresh_NewRunResetsBudget(t *testing.T) {
	rs := rules(testRule{
		RuleCore: RuleCore{Name: "fire", Service: "svc", Rate: 1, MaxAffected: 1},
		Error:    ErrorGeneric, Class: ClassTransient,
	})
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{RunID: "drill-1"}, "drill-1", rs)
	_, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	_, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil() // spent in drill-1

	// Same rule, new run: a fresh budget.
	k.Refresh(testConfig{RunID: "drill-2"}, "drill-2", rs)
	_, err = k.Fire(context.Background(), Target{Service: "svc"})
	assert.That(t, IsInjected(err)).True()
	assert.String(t, k.Config().RunID).Equal("drill-2")
}

// TestRefresh_StopDoesNotCancelInFlight pins the meaning of "stop": it stops
// producing NEW fires, it does not unwind an attempt already affected. The
// in-flight sleep completes and returns its decision; the next attempt matches
// nothing.
func TestRefresh_StopDoesNotCancelInFlight(t *testing.T) {
	const latency = 200 * time.Millisecond
	k := NewKernel[testConfig]()
	k.Refresh(testConfig{}, "", rules(testRule{
		RuleCore: RuleCore{Name: "slow", Service: "svc", Latency: latency},
	}))

	type result struct {
		d   Result
		err error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		d, err := k.Fire(context.Background(), Target{Service: "svc"})
		done <- result{d: d, err: err}
	}()

	// Wait until the attempt has claimed the rule — it is now inside the sleep.
	deadline := time.Now().Add(time.Second)
	for k.cur.Load().budgets.byName["slow"].affected.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the fire never started")
		}
		time.Sleep(time.Millisecond)
	}

	// Stop: remove the rule from the armed document.
	k.Refresh(testConfig{}, "", nil)

	r := <-done
	assert.Error(t, r.err).Nil()
	assert.That(t, r.d.Fired()).True()
	assert.That(t, time.Since(start) >= latency).True() // the sleep was not cut short

	// A call arriving after the stop is not fired at all.
	start = time.Now()
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
	assert.That(t, time.Since(start) < latency).True()
}

// TestKernel_UnarmedIsTransparent pins the unarmed kernel's contract: no
// document pushed means no rules, so Fire decides nothing (zero Result, nil
// error — the caller runs the real fn) and Config reports the zero document.
func TestKernel_UnarmedIsTransparent(t *testing.T) {
	k := NewKernel[testConfig]()
	d, err := k.Fire(context.Background(), Target{Service: "svc"})
	assert.Error(t, err).Nil()
	assert.That(t, d.Fired()).False()
	assert.That(t, d == Result{}).True()
	assert.String(t, k.Config().RunID).Equal("")
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
