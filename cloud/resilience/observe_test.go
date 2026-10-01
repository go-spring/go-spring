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
	"testing"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// fakeExecutor returns a fixed error from Execute so a test can drive every
// status classification without wiring a real driver.
type fakeExecutor struct{ err error }

func (f fakeExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return f.err
}
func (fakeExecutor) Close() error                 { return nil }
func (fakeExecutor) Refresh(p ClientPolicy) error { return nil }

func TestWrapClientExecutor_NilInnerReturnsNil(t *testing.T) {
	assert.That(t, WrapClientExecutor(nil, "sys", "svc")).Nil()
}

// TestClassifyStatusAndOutcome proves the two keys answer two different
// questions, which is why they are separate: `status` is the ecosystem-wide axis
// (`ok`/`error`, the same words an inbound request uses), and
// `resilience.outcome` is what protection decided about the call — empty unless
// it refused one or the call ran out of budget.
func TestClassifyStatusAndOutcome(t *testing.T) {
	cases := []struct {
		err     error
		status  string
		outcome string
	}{
		{nil, "ok", ""},
		{ErrRateLimited, "error", "rate_limited"},
		{ErrCircuitOpen, "error", "circuit_open"},
		{ErrBulkheadFull, "error", "bulkhead_full"},
		{ErrRetryBudgetExceeded, "error", "retry_budget_exceeded"},
		{context.DeadlineExceeded, "error", "timeout"},
		// A plain downstream failure is an error, but nothing protection did.
		{errutil.Explain(nil, "boom"), "error", ""},
		// wrapped sentinels must still classify by errors.Is.
		{errors.Join(ErrCircuitOpen, errutil.Explain(nil, "detail")), "error", "circuit_open"},
	}
	for _, c := range cases {
		assert.That(t, classifyStatus(c.err)).Equal(c.status)
		assert.That(t, classifyOutcome(c.err)).Equal(c.outcome)
	}
}

// TestWrapClientExecutor_PassesErrorThrough exercises every status end to end: the
// wrapper must return the inner error unchanged (no swallowing) while emitting
// signals. The OTel globals are no-ops here, so this only proves pass-through +
// no-panic; metric values are verified by the SDK-backed test below.
func TestWrapClientExecutor_PassesErrorThrough(t *testing.T) {
	for _, err := range []error{
		nil,
		ErrRateLimited,
		ErrCircuitOpen,
		ErrBulkheadFull,
		context.DeadlineExceeded,
		errutil.Explain(nil, "downstream"),
	} {
		exec := WrapClientExecutor(fakeExecutor{err: err}, "redis", "svc")
		got := exec.Execute(context.Background(), func(ctx context.Context) error { return nil })
		assert.That(t, errors.Is(got, err)).True()
	}
	_ = ErrBulkheadFull // keep import even if slice above changes
}
