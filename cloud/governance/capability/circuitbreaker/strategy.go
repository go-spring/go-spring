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

// strategy.go holds the four counting strategies behind the breaker (see
// [Strategy]): consecutive failures, error count, error rate, and slow calls.
// The Breaker (breaker.go) owns the open/half-open switching; each strategy owns
// what "failing enough" means for its own counting scheme and is built from a
// [Spec] by newCountingStrategy.

package circuitbreaker

import (
	"fmt"
	"time"
)

// countingStrategy is one strategy behind the breaker: it folds completed
// outcomes into its own counters and reports when the failure threshold is
// met. The Breaker owns the open/half-open switching; each strategy owns what
// "failing enough" means for its own counting scheme. Exactly one strategy runs
// per breaker — strategies do not stack (see [Spec]).
type countingStrategy interface {
	// record folds one completed outcome into the strategy's counters and
	// reports whether the circuit should open.
	record(success bool, duration time.Duration) (trip bool)

	// reset clears all counts as if nothing had been observed; the Breaker
	// calls it when the circuit closes.
	reset()
}

// newCountingStrategy builds the counting strategy a Spec asks for. Each
// strategy's constructor validates only the fields it reads — fields another
// strategy would read are not its business and are ignored. The strategy
// decides which fields count, not the other way around.
func newCountingStrategy(spec Spec) (countingStrategy, error) {
	switch spec.Strategy {
	case StrategyConsecutive:
		return newConsecutiveStrategy(spec)
	case StrategySlowCall:
		return newSlowCallStrategy(spec)
	case StrategyErrorCount:
		return newErrorCountStrategy(spec)
	case StrategyErrorRate:
		return newErrorRateStrategy(spec)
	}
	return nil, fmt.Errorf("circuitbreaker: unknown or empty strategy %q", spec.Strategy)
}

// consecutiveStrategy trips after FailureThreshold failures in a row; any
// success resets the count. It ignores duration and [Spec.Window]: it has no
// time dimension.
type consecutiveStrategy struct {
	failureThreshold int
	failures         int
}

func newConsecutiveStrategy(spec Spec) (countingStrategy, error) {
	if spec.FailureThreshold <= 0 {
		return nil, fmt.Errorf("circuitbreaker: consecutive requires a positive failure threshold, got %d", spec.FailureThreshold)
	}
	return &consecutiveStrategy{failureThreshold: spec.FailureThreshold}, nil
}

func (c *consecutiveStrategy) record(success bool, _ time.Duration) (trip bool) {
	if success {
		c.failures = 0
		return false
	}
	c.failures++
	return c.failures >= c.failureThreshold
}

func (c *consecutiveStrategy) reset() { c.failures = 0 }

// counts is one window's sample counts. Keeping the three numbers together
// makes the current and previous buckets symmetric and the decay arithmetic
// readable.
type counts struct {
	total     int
	failures  int
	slowCalls int
}

// rollingWindow is the rolling estimate the windowed strategies share. It keeps
// two buckets: cur accumulates the current interval, prev holds the interval
// that just ended. When an interval elapses, cur becomes prev and prev's counts
// then decay linearly as the new interval fills, so weighted blends
// prev×remaining + cur. It approximates a true sliding window well enough for
// trip decisions while needing no timers and no ring of buckets.
//
// A record that arrives two or more intervals late drops prev entirely: that
// bucket is too stale to describe the present.
type rollingWindow struct {
	minSamples int
	interval   time.Duration

	curStart time.Time
	elapsed  time.Duration // time since curStart, as of the last record

	cur, prev counts
}

// newRollingWindow builds the window a Spec asks for. A non-positive MinSamples
// becomes 1 (trip on the first sample) and a non-positive Window becomes one
// second; a negative Window is a configuration error.
func newRollingWindow(spec Spec) (rollingWindow, error) {
	if spec.Window < 0 {
		return rollingWindow{}, fmt.Errorf("circuitbreaker: negative window %v", spec.Window)
	}
	minSamples := spec.MinSamples
	if minSamples <= 0 {
		minSamples = 1
	}
	interval := spec.Window
	if interval <= 0 {
		interval = time.Second
	}
	return rollingWindow{minSamples: minSamples, interval: interval, curStart: time.Now()}, nil
}

// record counts one outcome in the current bucket, advancing the buckets first
// if the interval elapsed. Slowness is counted separately by markSlow, because
// only the strategy knows whether the duration is over its threshold.
func (w *rollingWindow) record(success bool) {
	now := time.Now()
	w.elapsed = now.Sub(w.curStart)
	if w.elapsed >= w.interval {
		if w.elapsed >= 2*w.interval {
			// The previous bucket is a whole interval stale: drop it entirely.
			w.prev = counts{}
		} else {
			w.prev = w.cur
		}
		w.cur = counts{}
		w.curStart = now
		w.elapsed = 0
	}
	w.cur.total++
	if !success {
		w.cur.failures++
	}
}

// markSlow counts the outcome just recorded as slow. Callers invoke it after
// record, so the count belongs to the same bucket.
func (w *rollingWindow) markSlow() { w.cur.slowCalls++ }

// weighted blends the buckets into the counts the strategies compare against
// their thresholds. The current bucket counts in full; the previous one is
// scaled by the share of the current interval still to come — 1 right after a
// roll, approaching 0 as the interval fills.
func (w *rollingWindow) weighted() (total, failures, slowCalls float64) {
	weight := float64(w.interval-w.elapsed) / float64(w.interval)
	total = float64(w.cur.total) + float64(w.prev.total)*weight
	failures = float64(w.cur.failures) + float64(w.prev.failures)*weight
	slowCalls = float64(w.cur.slowCalls) + float64(w.prev.slowCalls)*weight
	return total, failures, slowCalls
}

// reset drops both buckets and restarts the current interval; the Breaker calls
// it when the circuit closes, so a recovered breaker starts with fresh evidence.
func (w *rollingWindow) reset() {
	w.cur, w.prev = counts{}, counts{}
	w.curStart = time.Now()
	w.elapsed = 0
}

// errorRateStrategy trips when either arm reaches its threshold, once the window
// holds at least MinSamples estimated samples:
//
//   - failures/total >= FailureRate (the arm is off when FailureRate is 0);
//   - slowCalls/total >= SlowRate, where a slow call is any outcome above
//     SlowThreshold, successful or failed (the arm is off when SlowRate is 0).
//
// Both ratios share one window, so they answer two questions about the same
// traffic: how much failed, and how much was unacceptably slow. Counting slow
// failures in the slow ratio is the mainstream choice (resilience4j/Sentinel);
// it does not double a verdict, because the arms are independent OR signals.
type errorRateStrategy struct {
	failureRate   float64
	slowRate      float64
	slowThreshold time.Duration
	window        rollingWindow
}

// newErrorRateStrategy validates the fields this strategy reads: both ratio
// bounds, the slow pair (both halves or neither), and at least one active arm.
// A failure rate with no slow pair and a complete slow pair with FailureRate 0
// are both valid; both zero is not.
func newErrorRateStrategy(spec Spec) (countingStrategy, error) {
	if spec.FailureRate < 0 || spec.FailureRate > 1 {
		return nil, fmt.Errorf("circuitbreaker: failure rate %v outside [0, 1]", spec.FailureRate)
	}
	if (spec.SlowThreshold > 0) != (spec.SlowRate > 0) {
		return nil, fmt.Errorf("circuitbreaker: slow-call pair must set both SlowThreshold and SlowRate")
	}
	if spec.SlowRate < 0 || spec.SlowRate > 1 {
		return nil, fmt.Errorf("circuitbreaker: slow rate %v outside [0, 1]", spec.SlowRate)
	}
	if spec.FailureRate == 0 && spec.SlowRate == 0 {
		return nil, fmt.Errorf("circuitbreaker: error-rate requires a positive failure rate or a complete slow-call pair")
	}
	window, err := newRollingWindow(spec)
	if err != nil {
		return nil, err
	}
	return &errorRateStrategy{
		failureRate:   spec.FailureRate,
		slowRate:      spec.SlowRate,
		slowThreshold: spec.SlowThreshold,
		window:        window,
	}, nil
}

// record folds one outcome into the shared window, marks any outcome above the
// slow threshold (success or failure), then checks both arms against the
// decayed estimate. Samples below MinSamples never trip.
func (c *errorRateStrategy) record(success bool, duration time.Duration) (trip bool) {
	c.window.record(success)
	if c.slowThreshold > 0 && duration > c.slowThreshold {
		c.window.markSlow()
	}
	total, failures, slowCalls := c.window.weighted()
	// The sample gate reads the same decayed estimate as the ratios: raw
	// counts left over from a bucket that has almost decayed away are not
	// evidence enough to trip on.
	if total < float64(c.window.minSamples) {
		return false
	}
	if c.failureRate > 0 && failures/total >= c.failureRate {
		return true
	}
	if c.slowRate == 0 {
		return false // the slow arm is not configured
	}
	return slowCalls/total >= c.slowRate
}

func (c *errorRateStrategy) reset() { c.window.reset() }

// slowCallStrategy trips when slowCalls/total over the window reaches SlowRate;
// slowness is its only signal. A "slow call" is any outcome above
// SlowThreshold, successful or failed; fast failures still count in the
// denominator but never in the numerator, so they can only dilute the ratio.
type slowCallStrategy struct {
	slowRate      float64
	slowThreshold time.Duration
	window        rollingWindow
}

// newSlowCallStrategy validates the pair: both halves are required, and the
// rate must be in [0, 1].
func newSlowCallStrategy(spec Spec) (countingStrategy, error) {
	if (spec.SlowThreshold > 0) != (spec.SlowRate > 0) {
		return nil, fmt.Errorf("circuitbreaker: slow-call pair must set both SlowThreshold and SlowRate")
	}
	if spec.SlowRate < 0 || spec.SlowRate > 1 {
		return nil, fmt.Errorf("circuitbreaker: slow rate %v outside [0, 1]", spec.SlowRate)
	}
	if spec.SlowRate == 0 {
		return nil, fmt.Errorf("circuitbreaker: slow-call requires a complete slow-call pair")
	}
	window, err := newRollingWindow(spec)
	if err != nil {
		return nil, err
	}
	return &slowCallStrategy{
		slowRate:      spec.SlowRate,
		slowThreshold: spec.SlowThreshold,
		window:        window,
	}, nil
}

// record folds one outcome into the window, marks it slow when it is above the
// threshold, and compares the decayed slow ratio with SlowRate. Samples below
// MinSamples never trip.
func (c *slowCallStrategy) record(success bool, duration time.Duration) (trip bool) {
	c.window.record(success)
	if duration > c.slowThreshold {
		c.window.markSlow()
	}
	total, _, slowCalls := c.window.weighted()
	if total < float64(c.window.minSamples) {
		return false // not enough evidence yet
	}
	return slowCalls/total >= c.slowRate
}

func (c *slowCallStrategy) reset() { c.window.reset() }

// errorCountStrategy trips when the window's failure estimate reaches
// FailureThreshold. Successes interleave freely and never reset the count, but
// they do raise the total used by the MinSamples gate. Duration is ignored.
type errorCountStrategy struct {
	failureThreshold int
	window           rollingWindow
}

// newErrorCountStrategy validates the threshold; the window validates itself.
func newErrorCountStrategy(spec Spec) (countingStrategy, error) {
	if spec.FailureThreshold <= 0 {
		return nil, fmt.Errorf("circuitbreaker: error-count requires a positive failure threshold, got %d", spec.FailureThreshold)
	}
	window, err := newRollingWindow(spec)
	if err != nil {
		return nil, err
	}
	return &errorCountStrategy{failureThreshold: spec.FailureThreshold, window: window}, nil
}

// record folds one outcome into the window and compares the decayed failure
// count with FailureThreshold. Samples below MinSamples never trip.
func (c *errorCountStrategy) record(success bool, _ time.Duration) (trip bool) {
	c.window.record(success)
	total, failures, _ := c.window.weighted()
	if total < float64(c.window.minSamples) {
		return false
	}
	return failures >= float64(c.failureThreshold)
}

func (c *errorCountStrategy) reset() { c.window.reset() }
