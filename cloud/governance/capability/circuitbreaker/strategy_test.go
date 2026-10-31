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

package circuitbreaker

import (
	"testing"
	"time"
)

// The sample gate must read the same decayed estimate as the ratio. A bucket
// that has almost decayed away still holds raw counts, but they are no longer
// enough evidence to trip on.
func TestMinSamplesUsesDecayedEstimate(t *testing.T) {
	now := time.Now()
	w := rollingWindow{
		minSamples: 10,
		interval:   time.Minute,
		curStart:   now.Add(-59 * time.Second),
		prev:       counts{total: 100, failures: 100},
	}
	c := &errorCountStrategy{failureThreshold: 1, window: w}

	// One fresh success: the effective sample count is ~2.7 (100/60 + 1),
	// below MinSamples, so the near-decayed bucket must not trip the breaker.
	if c.record(true, 0) {
		t.Fatal("tripped on a bucket whose samples had decayed below MinSamples")
	}
}

func mustStrategy(t *testing.T, spec Spec) countingStrategy {
	t.Helper()
	s, err := newCountingStrategy(spec)
	if err != nil {
		t.Fatalf("newCountingStrategy(%+v): %v", spec, err)
	}
	return s
}

func TestConsecutiveStrategy(t *testing.T) {
	s := mustStrategy(t, Spec{Strategy: StrategyConsecutive, FailureThreshold: 3})

	if s.record(false, 0) {
		t.Fatal("tripped before the threshold")
	}
	if s.record(false, 0) {
		t.Fatal("tripped before the threshold")
	}
	if s.record(true, 0) {
		t.Fatal("a success reported a trip")
	}
	if s.record(false, 0) {
		t.Fatal("the success did not reset the run")
	}
	if s.record(false, 0) {
		t.Fatal("the success did not reset the run")
	}
	if !s.record(false, 0) {
		t.Fatal("did not trip at the threshold")
	}

	s.reset()
	if s.record(false, 0) {
		t.Fatal("reset did not clear the run")
	}
}

func TestErrorCountStrategy(t *testing.T) {
	s := mustStrategy(t, Spec{
		Strategy:         StrategyErrorCount,
		FailureThreshold: 2,
		MinSamples:       3,
		Window:           time.Minute,
	})

	if s.record(false, 0) {
		t.Fatal("tripped below MinSamples")
	}
	if s.record(true, 0) { // successes do not reset the count
		t.Fatal("success reported a trip")
	}
	if !s.record(false, 0) {
		t.Fatal("did not trip at the failure threshold")
	}

	s.reset()
	if s.record(false, 0) {
		t.Fatal("reset did not clear the counts")
	}
}

func TestErrorRateStrategyFailureArm(t *testing.T) {
	s := mustStrategy(t, Spec{
		Strategy:     StrategyErrorRate,
		FailureRate:  0.5,
		MinSamples:   2,
		Window:       time.Minute,
		OpenDuration: time.Hour,
	})

	if s.record(true, 0) {
		t.Fatal("tripped on a single success")
	}
	if !s.record(false, 0) { // 1/2 reaches 50%
		t.Fatal("did not trip on the failure ratio")
	}
}

func TestErrorRateStrategySlowArmCountsBothOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		success bool
	}{
		{"slow success", true},
		{"slow failure", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mustStrategy(t, Spec{
				Strategy:      StrategyErrorRate,
				FailureRate:   0, // the failure arm is off
				SlowThreshold: 10 * time.Millisecond,
				SlowRate:      0.5,
				MinSamples:    2,
				Window:        time.Minute,
			})
			s.record(true, 0) // a fast sample in the denominator
			if !s.record(tc.success, 20*time.Millisecond) {
				t.Fatal("a slow outcome did not feed the slow arm")
			}
		})
	}
}

func TestErrorRateFastFailuresDoNotFeedSlowArm(t *testing.T) {
	s := mustStrategy(t, Spec{
		Strategy:      StrategyErrorRate,
		FailureRate:   0, // only the slow arm is active
		SlowThreshold: 10 * time.Millisecond,
		SlowRate:      0.5,
		MinSamples:    2,
		Window:        time.Minute,
	})
	s.record(false, 0)
	if s.record(false, 0) {
		t.Fatal("fast failures counted as slow")
	}
}

func TestErrorRateStrategyMinSamples(t *testing.T) {
	s := mustStrategy(t, Spec{
		Strategy:     StrategyErrorRate,
		FailureRate:  0.5,
		MinSamples:   3,
		Window:       time.Minute,
		OpenDuration: time.Hour,
	})
	for i := 0; i < 2; i++ {
		if s.record(false, 0) {
			t.Fatalf("tripped at failure %d, below MinSamples", i+1)
		}
	}
	if !s.record(false, 0) {
		t.Fatal("did not trip once MinSamples was reached")
	}
}

func TestErrorRateSlowArmDisabled(t *testing.T) {
	s := mustStrategy(t, Spec{
		Strategy:     StrategyErrorRate,
		FailureRate:  0.9,
		MinSamples:   2,
		Window:       time.Minute,
		OpenDuration: time.Hour,
	})
	// Both samples are far above any plausible slow threshold, but SlowRate is
	// 0, so the slow arm must be ignored.
	s.record(true, time.Hour)
	if s.record(true, time.Hour) {
		t.Fatal("the slow arm tripped while SlowRate was 0")
	}
}

func TestSlowCallStrategy(t *testing.T) {
	t.Run("fast failures never trip", func(t *testing.T) {
		s := mustStrategy(t, Spec{
			Strategy:      StrategySlowCall,
			SlowThreshold: 10 * time.Millisecond,
			SlowRate:      0.5,
			MinSamples:    2,
			Window:        time.Minute,
		})
		s.record(false, 0)
		if s.record(false, 0) {
			t.Fatal("fast failures tripped the slow-call strategy")
		}
	})

	for _, tc := range []struct {
		name    string
		success bool
	}{
		{"slow success", true},
		{"slow failure", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := mustStrategy(t, Spec{
				Strategy:      StrategySlowCall,
				SlowThreshold: 10 * time.Millisecond,
				SlowRate:      0.5,
				MinSamples:    2,
				Window:        time.Minute,
			})
			s.record(true, 0)
			if !s.record(tc.success, 20*time.Millisecond) {
				t.Fatal("a slow call did not trip the strategy")
			}
		})
	}

	t.Run("threshold is strict", func(t *testing.T) {
		s := mustStrategy(t, Spec{
			Strategy:      StrategySlowCall,
			SlowThreshold: 10 * time.Millisecond,
			SlowRate:      0.5,
			MinSamples:    2,
			Window:        time.Minute,
		})
		s.record(true, 10*time.Millisecond)
		if s.record(true, 10*time.Millisecond) {
			t.Fatal("a call exactly at the threshold counted as slow")
		}
	})
}

func TestRollingWindowRollsIntoPreviousBucket(t *testing.T) {
	w := rollingWindow{minSamples: 1, interval: time.Second, curStart: time.Now()}
	w.record(true)
	w.markSlow()

	// Force the current interval to elapse, then record once more.
	w.curStart = time.Now().Add(-time.Second - time.Millisecond)
	w.record(false)

	if want := (counts{total: 1, slowCalls: 1}); w.prev != want {
		t.Fatalf("prev = %+v, want %+v", w.prev, want)
	}
	if want := (counts{total: 1, failures: 1}); w.cur != want {
		t.Fatalf("cur = %+v, want %+v", w.cur, want)
	}
	// The new interval just started, so prev carries full weight.
	total, failures, slowCalls := w.weighted()
	if total != 2 || failures != 1 || slowCalls != 1 {
		t.Fatalf("weighted = (%v, %v, %v), want (2, 1, 1)", total, failures, slowCalls)
	}
}

func TestRollingWindowDropsStalePreviousBucket(t *testing.T) {
	w := rollingWindow{minSamples: 1, interval: time.Second, curStart: time.Now()}
	w.record(false)

	// Two intervals later both buckets are stale: prev must be dropped whole.
	w.curStart = time.Now().Add(-2*time.Second - time.Millisecond)
	w.record(false)

	if w.prev != (counts{}) {
		t.Fatalf("prev = %+v, want zero", w.prev)
	}
	if want := (counts{total: 1, failures: 1}); w.cur != want {
		t.Fatalf("cur = %+v, want %+v", w.cur, want)
	}
}

func TestRollingWindowWeightedDecaysPreviousBucket(t *testing.T) {
	w := rollingWindow{
		interval: time.Second,
		elapsed:  500 * time.Millisecond, // half the interval remains
		cur:      counts{total: 10, failures: 2, slowCalls: 1},
		prev:     counts{total: 20, failures: 4, slowCalls: 2},
	}
	total, failures, slowCalls := w.weighted()
	if total != 20 || failures != 4 || slowCalls != 2 {
		t.Fatalf("weighted = (%v, %v, %v), want (20, 4, 2)", total, failures, slowCalls)
	}
}

func TestRollingWindowReset(t *testing.T) {
	w := rollingWindow{
		minSamples: 1,
		interval:   time.Minute,
		elapsed:    time.Second,
		cur:        counts{total: 5, failures: 2, slowCalls: 1},
		prev:       counts{total: 7, failures: 3, slowCalls: 2},
	}
	w.reset()
	if w.cur != (counts{}) || w.prev != (counts{}) || w.elapsed != 0 {
		t.Fatalf("reset left state behind: cur=%+v prev=%+v elapsed=%v", w.cur, w.prev, w.elapsed)
	}
	if total, failures, slowCalls := w.weighted(); total != 0 || failures != 0 || slowCalls != 0 {
		t.Fatalf("weighted after reset = (%v, %v, %v), want zero", total, failures, slowCalls)
	}
}

func TestNewStrategyValidation(t *testing.T) {
	for name, spec := range map[string]Spec{
		"consecutive zero threshold": {Strategy: StrategyConsecutive},
		"consecutive negative":       {Strategy: StrategyConsecutive, FailureThreshold: -1},
		"error-count zero":           {Strategy: StrategyErrorCount},
		"error-count negative":       {Strategy: StrategyErrorCount, FailureThreshold: -1},
		"error-rate no arm":          {Strategy: StrategyErrorRate},
		"error-rate negative rate":   {Strategy: StrategyErrorRate, FailureRate: -0.1},
		"error-rate rate above one":  {Strategy: StrategyErrorRate, FailureRate: 1.1},
		"error-rate half pair":       {Strategy: StrategyErrorRate, SlowThreshold: time.Second},
		"error-rate rate no pair":    {Strategy: StrategyErrorRate, SlowRate: 0.5},
		"slow-call no pair":          {Strategy: StrategySlowCall},
		"slow-call threshold only":   {Strategy: StrategySlowCall, SlowThreshold: time.Second},
		"slow-call rate only":        {Strategy: StrategySlowCall, SlowRate: 0.5},
		"slow-call rate above one":   {Strategy: StrategySlowCall, SlowThreshold: time.Second, SlowRate: 1.1},
		"unknown strategy":           {Strategy: "bogus"},
	} {
		if s, err := newCountingStrategy(spec); err == nil || s != nil {
			t.Errorf("%s: newCountingStrategy returned %v, %v", name, s, err)
		}
	}
}
