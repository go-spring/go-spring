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

// Package ratelimit is the direction-neutral rate-limit stage: the counting
// algorithms, the [Policy] a rule writes, the [Limiter] contract a counter
// implements, the [Provider] an implementation contributes, and the [Factory]
// that creates counters from them.
//
// Both call directions charge through it — outbound
// (go-spring.org/cloud/governance/dataplane/outbound/resilience) and inbound
// (go-spring.org/cloud/governance/dataplane/inbound/admission) — so the same
// rule counts a request the same way whichever way it travels. A counter keeps
// its state in memory when the process-local provider builds it, and outside
// the process when a shared store does.
//
// The package sits under the governance theme because rate limiting is a
// governance mechanism, but the counting code needs nothing beyond the standard
// library (plus stdlib/timeutil for ctx-aware sleeping), so a caller pulls in
// no resilience semantics to count with it.
//
// Three algorithms trade evenness for state: token-bucket, the default, is a
// port of Guava's SmoothRateLimiter — a charge is granted at the head of the
// pipeline and its cost charged to the next request, with an optional
// cold-start warm-up that prices permits above a warm threshold at up to
// coldFactor times their steady price; sliding-window smooths the window
// boundary with a weighted two-window estimate; fixed-window is the cheapest
// per-window cap and accepts a burst across the boundary.
//
// All counters are safe for concurrent use.
package ratelimit
