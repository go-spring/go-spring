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

// executor_default_retry.go is the retry organ of the bundled default
// executor: retry classification ([ClientPolicy.ShouldRetry]) and the
// exponential backoff with jitter that pace attempts.

package resilience

import (
	"errors"
	"math/rand"
	"time"
)

// ShouldRetry reports whether err ought to trigger another attempt. The
// classification is shared by every driver so retry decisions cannot drift
// between them: an error implementing [Retryable] opts itself in or out
// explicitly (the caller knows the concrete error type — e.g. a non-idempotent
// write failure opts out); every other non-nil error retries.
func (p ClientPolicy) ShouldRetry(err error) bool {
	if err == nil {
		return false
	}
	var r Retryable
	if errors.As(err, &r) {
		return r.Retryable()
	}
	return true
}

// Backoff returns the sleep duration before the retry following the
// (0-indexed) attempt-th failure, applying exponential growth and jitter per p.
//
// The schedule grows [ClientPolicy.InitialInterval] by [ClientPolicy.Multiplier] each step
// up to [ClientPolicy.MaxInterval], then decorrelates concurrent callers by ±
// [ClientPolicy.RandomizationFactor]. A zero InitialInterval yields 0 (no backoff),
// so a policy that sets only [ClientPolicy.MaxRetries] retries back to back.
// Both drivers call this so the math cannot drift.
func (p ClientPolicy) Backoff(attempt int) time.Duration {
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
