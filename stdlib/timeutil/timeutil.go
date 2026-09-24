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

// Package timeutil provides small time-related helpers that the Go standard
// library does not: context-aware waiting.
package timeutil

import (
	"context"
	"time"
)

// Sleep waits for d or until ctx is done, and reports whether the full
// duration elapsed: false means ctx ended first. A d of zero or less returns
// true immediately (after the usual ctx check).
//
// It is the context-aware counterpart of time.Sleep: a goroutine pacing a
// retry or re-poll loop sleeps in cancellable units, so shutdown does not
// wait out a long sleep that no longer matters.
func Sleep(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
