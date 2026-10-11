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

// Package ratelimit holds the rate-limit counting algorithms shared by the
// resilience stage (go-spring.org/cloud/resilience) and any process-local
// resilience.ClusterLimiter store built on top of them. It is a leaf package: the
// counters depend on nothing beyond the standard library (plus stdlib/timeutil
// for ctx-aware sleeping), so callers pull in no resilience semantics to count
// with them.
//
// The counters trade smoothness for state. [FixedWindow] is the simplest and
// cheapest but admits a 2x burst at window boundaries; [SlidingWindow] smooths
// that with a weighted second window; [TokenBucket] smooths fully, refilling
// continuously, and adds a cold-start warm-up; [LeakyBucket] is the dual of
// the token bucket, bounding by constant drain instead of refill; [GCRA] is
// the token bucket expressed as one stored timestamp — the shape that ports
// to an atomic store script with the least state per key.
//
// All counters are safe for concurrent use.
package ratelimit
