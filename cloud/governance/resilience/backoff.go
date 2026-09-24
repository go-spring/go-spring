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

package resilience

import (
	"context"
	"errors"
	"go-spring.org/stdlib/timeutil"
	"math/rand"
	"time"
)

// ShouldRetry reports whether err ought to trigger another attempt under p. The
// resolution order is intentional and shared by every driver so retry
// classification cannot drift between them:
//
//  1. If err implements [Retryable], its verdict wins — the caller knows the
//     concrete error type and opts in/out explicitly.
//  2. Otherwise, if [Policy.RetryPredicate] is set, it decides.
//  3. Otherwise (nil predicate), every non-nil error retries — the historical
//     behavior, preserved so existing configs keep retrying as before.
//
// Both the default and the sentinel driver call this so their retry decisions
// stay in lockstep.
func (p Policy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var r Retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	if p.RetryPredicate != nil {
		return p.RetryPredicate(err)
	}
	return true
}

// Backoff returns the sleep duration before the retry following the
// (0-indexed) attempt-th failure, applying exponential growth and jitter per p.
//
// The schedule grows [Policy.InitialInterval] by [Policy.Multiplier] each step
// up to [Policy.MaxInterval], then decorrelates concurrent callers by ±
// [Policy.RandomizationFactor]. A zero InitialInterval yields 0 (no backoff),
// which is how a legacy Policy with only MaxRetries set keeps its back-to-back
// retry behavior. Both drivers call this so the math cannot drift.
func (p Policy) Backoff(attempt int) time.Duration {
	if p.InitialInterval <= 0 {
		return 0
	}
	mult := p.Multiplier
	if mult <= 0 {
		mult = 1
	}
	d := float64(p.InitialInterval)
	for range attempt {
		d *= mult
		if p.MaxInterval > 0 && d > float64(p.MaxInterval) {
			d = float64(p.MaxInterval)
			break
		}
	}
	if p.RandomizationFactor > 0 {
		// ± factor*d: center the jitter on d so the average backoff stays d.
		delta := d * p.RandomizationFactor
		d = d + (rand.Float64()*2-1)*delta
	}
	return time.Duration(d)
}

// SleepFor blocks for d respecting ctx. It returns false if ctx fired during
// the sleep, signalling the caller to stop the retry loop. A non-positive d is
// a no-op that returns true (backoff disabled). Both drivers use it to pace
// retries with a single cancellation contract.
func SleepFor(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	return timeutil.Sleep(ctx, d)
}
