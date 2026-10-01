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

package observability_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
)

// TestRecorderFromPlainContextIsNil proves a context that carries no Recorder
// reports nil, which is what lets the governance loop record unconditionally.
func TestRecorderFromPlainContextIsNil(t *testing.T) {
	assert.That(t, observability.RecorderFrom(context.Background()) == nil).True()
}

// TestWithRecorderInstallsFreshRecorder proves the installer returns the very
// Recorder it put on the context, so the emitter needs no second lookup.
func TestWithRecorderInstallsFreshRecorder(t *testing.T) {
	ctx, rec := observability.WithRecorder(context.Background())
	assert.That(t, observability.RecorderFrom(ctx) == rec).True()
}

// TestRecorderRecordsAttemptsInOrder proves attempts keep their order, their
// status and their own error, which is what lets the emitter tell a recovered
// retry from a failure and bucket each try by outcome.
func TestRecorderRecordsAttemptsInOrder(t *testing.T) {
	_, rec := observability.WithRecorder(context.Background())
	boom := errors.New("boom")
	rec.AddAttempt(10*time.Millisecond, "error", boom)
	rec.AddAttempt(20*time.Millisecond, "success", nil)

	got := rec.Attempts()
	assert.Number(t, len(got)).Equal(2)
	assert.Number(t, got[0].Duration).Equal(10 * time.Millisecond)
	assert.String(t, got[0].Status).Equal("error")
	assert.Error(t, got[0].Err).Is(boom)
	assert.Number(t, got[1].Duration).Equal(20 * time.Millisecond)
	assert.String(t, got[1].Status).Equal("success")
	assert.Error(t, got[1].Err).Nil()
}

// TestRecorderSumsBackoff proves backoff accumulates across the gaps between
// attempts, kept apart from attempt durations so the retry policy's own cost is
// readable without subtraction.
func TestRecorderSumsBackoff(t *testing.T) {
	_, rec := observability.WithRecorder(context.Background())
	rec.AddBackoff(100 * time.Millisecond)
	rec.AddBackoff(200 * time.Millisecond)
	assert.Number(t, rec.Backoff()).Equal(300 * time.Millisecond)
}

// TestRecorderZeroValueReadsEmpty proves an untouched Recorder reports empty
// rather than panicking, so a call that was rejected before any attempt — a
// rate-limit or circuit-open rejection — reads as zero attempts.
func TestRecorderZeroValueReadsEmpty(t *testing.T) {
	_, rec := observability.WithRecorder(context.Background())
	assert.Number(t, len(rec.Attempts())).Equal(0)
	assert.Number(t, rec.Backoff()).Equal(time.Duration(0))
}

// TestNilRecorderIsSafe proves every write is a no-op on a nil Recorder, which
// is the ordinary state of a call with no governance layer.
func TestNilRecorderIsSafe(t *testing.T) {
	var rec *observability.Recorder
	rec.AddAttempt(time.Second, "error", errors.New("boom"))
	rec.AddBackoff(time.Second)
	assert.Number(t, len(rec.Attempts())).Equal(0)
	assert.Number(t, rec.Backoff()).Equal(time.Duration(0))
}

// TestNestedRecorderIsFresh proves a derivation gets its own Recorder instead of
// sharing the parent's — the installer does not reuse one already present — so a
// call nested inside another does not append its attempts to the outer's record.
func TestNestedRecorderIsFresh(t *testing.T) {
	outer, outerRec := observability.WithRecorder(context.Background())
	_, innerRec := observability.WithRecorder(outer)

	assert.That(t, innerRec == outerRec).False()
	innerRec.AddAttempt(time.Second, "success", nil)
	assert.Number(t, len(outerRec.Attempts())).Equal(0)
	assert.Number(t, len(innerRec.Attempts())).Equal(1)
}

// TestRecorderFromTakesNearest proves the lookup resolves to the innermost
// Recorder on the chain, which is what lets nesting work without either call
// reaching for the other's record.
func TestRecorderFromTakesNearest(t *testing.T) {
	outer, _ := observability.WithRecorder(context.Background())
	inner, innerRec := observability.WithRecorder(outer)

	assert.That(t, observability.RecorderFrom(inner) == innerRec).True()
	assert.That(t, observability.RecorderFrom(outer) != innerRec).True()
}
