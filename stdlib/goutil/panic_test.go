/*
 * Copyright 2024 The Go-Spring Authors.
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

package goutil

import (
	"context"
	"strings"
	"testing"
)

// withTestOnPanic swaps OnPanic for the duration of one test and restores it
// afterwards: it is a package-global slot, so tests must not leak their
// handler into later tests.
func withTestOnPanic(t *testing.T, fn func(context.Context, PanicInfo)) {
	t.Helper()
	old := OnPanic
	OnPanic = fn
	t.Cleanup(func() { OnPanic = old })
}

// TestReportPanicNilDispatches pins the contract that ReportPanic does not
// filter the recovered value: nil is dispatched as-is, callers that may hold
// a nil value check it themselves.
func TestReportPanicNilDispatches(t *testing.T) {
	var got PanicInfo
	withTestOnPanic(t, func(_ context.Context, info PanicInfo) { got = info })
	ReportPanic(context.Background(), nil)
	if got.Panic != nil {
		t.Fatal("nil recovered value must be reported as-is")
	}
}

// TestReportPanicReachesHandler proves the shared entry point routes to the
// installed handler with the panic value and a stack that still contains the
// panicking frames.
func TestReportPanicReachesHandler(t *testing.T) {
	var got PanicInfo
	withTestOnPanic(t, func(_ context.Context, info PanicInfo) { got = info })

	ReportPanic(context.Background(), "boom")
	if got.Panic != "boom" {
		t.Fatalf("handler should see the panic value, got %v", got.Panic)
	}
	if !strings.Contains(string(got.Stack), "TestReportPanicReachesHandler") {
		t.Fatalf("stack should contain the reporting frame: %s", got.Stack)
	}
}

// TestSafeRunConvertsPanic proves a panic becomes an error on the normal
// return path (naming the panic value, stack goes to OnPanic instead) — the
// property scheduler jobs and message handlers rely on.
func TestSafeRunConvertsPanic(t *testing.T) {
	var reported any
	withTestOnPanic(t, func(_ context.Context, info PanicInfo) { reported = info.Panic })

	err := SafeRun(context.Background(), func(context.Context) error {
		panic("boom")
	})
	if err == nil {
		t.Fatal("expected error from panicked SafeRun")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error should name the panic value: %v", err)
	}
	if strings.Contains(err.Error(), "goroutine") {
		t.Fatalf("error should not embed the stack: %v", err)
	}
	if reported != "boom" {
		t.Fatalf("OnPanic should have been invoked, got %v", reported)
	}

	// The clean path passes the error through unchanged.
	sentinel := err
	got := SafeRun(context.Background(), func(context.Context) error { return sentinel })
	if got != sentinel {
		t.Fatalf("clean path must pass the error through: %v", got)
	}
}

// TestGoStillRecovers pins the launcher behavior: a panicking goroutine is
// recovered and does not crash the process.
func TestGoStillRecovers(t *testing.T) {
	s := Go(context.Background(), func(context.Context) {
		panic("worker boom")
	}, InheritCancel)
	s.Wait() // returning at all proves the panic was recovered
}
