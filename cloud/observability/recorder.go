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

// recorder.go carries the per-call accumulator that the retry loop inside a
// protected call writes and the single emitter outside it reads. The two are
// deliberately apart: the loop knows facts nothing else can see (how many
// attempts ran, what each cost, how long the backoffs were) but must not emit;
// the emitter sits at the one place a call can be reported whole.
//
// A Recorder belongs to exactly ONE call and travels on that call's context. It
// is written only by the governance loop, on the calling goroutine, so it holds
// no lock: a nested protected call derives a fresh context and gets a fresh
// Recorder instead of sharing this one.

package observability

import (
	"context"
	"time"
)

// recorderKey is the private key under which a context carries a Recorder. An
// unexported empty-struct type keeps it collision-free.
type recorderKey struct{}

// Attempt is one downstream try made inside a protected call, recorded where it
// happened. Duration is the time the try itself took — not including the
// backoff that followed it, which is tracked separately on the Recorder so that
// "how slow is the downstream" and "how much did our retry policy add" never
// have to be told apart by subtraction at read time.
//
// Status names the try's outcome and Err carries its error, nil on success. The
// pair is supplied by the writer rather than derived here: the vocabulary for an
// outcome belongs to whoever owns the errors (the resilience sentinels), and
// classifying them here would make this package depend on that owner. Err is
// what a failure log reads from; Status is what the attempt histogram buckets by.
type Attempt struct {
	Duration time.Duration
	Status   string
	Err      error
}

// Recorder accumulates the attempts of one protected call, in order, plus the
// total time spent sleeping between them.
type Recorder struct {
	attempts []Attempt
	backoff  time.Duration
}

// NewRecorder returns an empty Recorder.
func NewRecorder() *Recorder { return &Recorder{} }

// WithRecorder returns a context carrying a fresh Recorder, and that Recorder.
// Returning it alongside the context is what spares the emitter a second lookup:
// it installs the Recorder before handing the context inward and reads it back
// once the call returns.
//
// It installs unconditionally — it does NOT reuse a Recorder the context already
// carries. That is deliberate: a protected call nested inside another must get
// its own, so each call's attempts are recorded apart from the one it is nested
// in. Because a context value is looked up from the innermost layer outward,
// [RecorderFrom] then finds the nearest one, which is the nesting's own.
func WithRecorder(ctx context.Context) (context.Context, *Recorder) {
	r := NewRecorder()
	return context.WithValue(ctx, recorderKey{}, r), r
}

// RecorderFrom returns the Recorder nearest to ctx — the innermost one on the
// derivation chain — or nil when it carries none. A nil return is the ordinary
// case for a call that ran without a governance layer installing one, and
// callers must treat it as "record nothing" rather than as an error.
//
// A caller that owns a call reads this ONCE, before handing the context inward,
// and keeps the pointer: a deeper nested call installs its own Recorder, and a
// per-write lookup would hand that inner Recorder the outer call's records.
func RecorderFrom(ctx context.Context) *Recorder {
	if r, ok := ctx.Value(recorderKey{}).(*Recorder); ok {
		return r
	}
	return nil
}

// AddAttempt appends one attempt. A nil Recorder is a no-op, so the governance
// loop can record unconditionally.
func (r *Recorder) AddAttempt(d time.Duration, status string, err error) {
	if r == nil {
		return
	}
	r.attempts = append(r.attempts, Attempt{Duration: d, Status: status, Err: err})
}

// AddBackoff adds d to the total time spent sleeping between attempts. A nil
// Recorder is a no-op.
func (r *Recorder) AddBackoff(d time.Duration) {
	if r == nil {
		return
	}
	r.backoff += d
}

// Attempts returns the attempts recorded so far, oldest first, or nil when the
// call made none — a rejection that returned before the first attempt. The
// result is owned by the Recorder and must not be modified.
func (r *Recorder) Attempts() []Attempt {
	if r == nil {
		return nil
	}
	return r.attempts
}

// Backoff returns the total time spent sleeping between attempts. It is the
// part of a call's wall time that the downstream never saw, so a call's total
// minus the sum of its attempts minus this is the framework's own overhead.
func (r *Recorder) Backoff() time.Duration {
	if r == nil {
		return 0
	}
	return r.backoff
}
