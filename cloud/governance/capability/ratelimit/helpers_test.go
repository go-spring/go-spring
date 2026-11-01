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

package ratelimit

import (
	"context"
	"time"
)

// allow charges n units through the [Limiter] contract and returns the verdict.
func allow(l Limiter, p Policy, n int) bool {
	ok, _ := l.Allow(context.Background(), p, n)
	return ok
}

// windowPolicy asks for limit events per window — how the window algorithms'
// tests state their budget.
func windowPolicy(limit float64, window time.Duration) Policy {
	return Policy{RateLimit: limit / window.Seconds(), Window: window}
}
