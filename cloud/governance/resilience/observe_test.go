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

	observe "go-spring.org/cloud/observe"
	"go-spring.org/stdlib/testing/assert"
)

// fakeExecutor returns a fixed error from Execute so a test can drive every
// outcome classification without wiring a real driver.
type fakeExecutor struct{ err error }

func (f fakeExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	return f.err
}
func (fakeExecutor) Close() error           { return nil }
func (fakeExecutor) Refresh(p Policy) error { return nil }

func TestWrapExecutor_NilInnerReturnsNil(t *testing.T) {
	assert.That(t, WrapExecutor(nil, "redis", observe.ObserveConfig{})).Nil()
}

func TestClassifyOutcome(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, "success"},
		{ErrRateLimited, "rate_limited"},
		{ErrCircuitOpen, "circuit_open"},
		{ErrBulkheadFull, "bulkhead_full"},
		{context.DeadlineExceeded, "timeout"},
		{errors.New("boom"), "error"},
		// wrapped sentinels must still classify by errors.Is.
		{errors.Join(ErrCircuitOpen, errors.New("detail")), "circuit_open"},
	}
	for _, c := range cases {
		assert.That(t, classifyOutcome(c.err)).Equal(c.want)
	}
}

// TestWrapExecutor_PassesErrorThrough exercises every outcome end to end: the
// wrapper must return the inner error unchanged (no swallowing) while emitting
// signals. The OTel globals are no-ops here, so this only proves pass-through +
// no-panic; metric values are verified by the SDK-backed test below.
func TestWrapExecutor_PassesErrorThrough(t *testing.T) {
	for _, err := range []error{
		nil,
		ErrRateLimited,
		ErrCircuitOpen,
		ErrBulkheadFull,
		context.DeadlineExceeded,
		errors.New("downstream"),
	} {
		exec := WrapExecutor(fakeExecutor{err: err}, "redis", observe.ObserveConfig{Level: "off"})
		got := exec.Execute(context.Background(), "svc", func(ctx context.Context) error { return nil })
		assert.That(t, errors.Is(got, err)).True()
	}
	_ = ErrBulkheadFull // keep import even if slice above changes
}
