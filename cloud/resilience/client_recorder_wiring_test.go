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
	"go-spring.org/cloud/chain"
	"testing"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// TestGovernanceLoopRecordsAttempts proves the retry loop writes each attempt —
// its outcome and its own cost — and the sleep between attempts into the
// context-carried recorder, which is the only source the outer emitter has.
func TestGovernanceLoopRecordsAttempts(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{MaxRetries: 2, InitialInterval: 5 * time.Millisecond})
	boom := errutil.Explain(nil, "transient")

	ctx, rec := observability.WithRecorder(context.Background())
	var calls int
	err := e.Execute(ctx, func(context.Context) error {
		calls++
		if calls < 3 {
			return boom
		}
		return nil
	})
	assert.Error(t, err).Nil()

	got := rec.Attempts()
	assert.Number(t, len(got)).Equal(3)
	assert.Error(t, got[0].Err).Is(boom)
	assert.Error(t, got[1].Err).Is(boom)
	assert.Error(t, got[2].Err).Nil()
	// The status is classified by the loop, so the attempt histogram has its
	// bucket without the emitter knowing this package's sentinels.
	assert.String(t, got[0].Status).Equal("error")
	assert.String(t, got[1].Status).Equal("error")
	assert.String(t, got[2].Status).Equal("ok")
	// Two gaps between three attempts, each the (Multiplier==0 -> 1) fixed
	// interval. The recorder holds the intended sleep, so the assertion is exact
	// rather than timing-dependent.
	assert.Number(t, rec.Backoff()).Equal(10 * time.Millisecond)
}

// TestRejectionRecordsNoAttempt proves a call a protection stage rejects before
// any attempt runs leaves the recorder empty rather than inventing an attempt —
// the emitter reads that emptiness as "rejected, downstream untouched".
func TestRejectionRecordsNoAttempt(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{RateLimit: 1, Burst: 1})
	ctx, rec := observability.WithRecorder(context.Background())

	// First call spends the burst; the second is rejected before running fn.
	assert.Error(t, e.Execute(ctx, func(context.Context) error { return nil })).Nil()
	assert.Error(t, e.Execute(ctx, func(context.Context) error { return nil })).Is(chain.ErrRateLimited)

	// The recorder is per call: the rejection ran on a fresh one.
	_, rec2 := observability.WithRecorder(context.Background())
	assert.Number(t, len(rec.Attempts())).Equal(1)
	assert.Number(t, len(rec2.Attempts())).Equal(0)
}

// TestNestedCallDoesNotHijackOuterRecorder pins the load-bearing detail: the
// outer loop holds the Recorder it looked up at entry, so a protected call
// nested inside its fn installs and writes its own without stealing the outer
// call's records. Were the lookup per-write instead of once-at-entry, the inner
// Recorder would take the outer's attempt.
func TestNestedCallDoesNotHijackOuterRecorder(t *testing.T) {
	outer := newBuiltin(t, ClientPolicy{})
	inner := newBuiltin(t, ClientPolicy{})

	ctx, outerRec := observability.WithRecorder(context.Background())
	err := outer.Execute(ctx, func(innerCtx context.Context) error {
		innerCtx, innerRec := observability.WithRecorder(innerCtx)
		assert.Error(t, inner.Execute(innerCtx, func(context.Context) error { return nil })).Nil()
		assert.Number(t, len(innerRec.Attempts())).Equal(1)
		assert.String(t, innerRec.Attempts()[0].Status).Equal("ok")
		return errutil.Explain(nil, "outer fails")
	})

	assert.Error(t, err).NotNil()
	assert.Number(t, len(outerRec.Attempts())).Equal(1)
	assert.Error(t, outerRec.Attempts()[0].Err).NotNil()
}

// TestExecuteWithoutRecorderIsSafe proves an executor used with a bare context —
// no governance layer installing a recorder — runs unchanged: recording is
// additive and never required.
func TestExecuteWithoutRecorderIsSafe(t *testing.T) {
	e := newBuiltin(t, ClientPolicy{MaxRetries: 1, InitialInterval: time.Millisecond})
	var calls int
	err := e.Execute(context.Background(), func(context.Context) error {
		calls++
		return errutil.Explain(nil, "transient")
	})
	assert.Error(t, err).NotNil()
	assert.Number(t, calls).Equal(2)
}
