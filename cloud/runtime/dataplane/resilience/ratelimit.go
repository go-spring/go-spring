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

// ratelimit.go is the outbound view of the shared rate-limit stage: the
// executor's own budget ([go-spring.org/cloud/runtime/dataplane/ratelimit.Budget]) and the
// [ClusterLimiter] seam (where a cross-replica budget lives). The counting
// algorithms, the [RateSpec] vocabulary and the local budget live in
// go-spring.org/cloud/runtime/dataplane/ratelimit, a leaf package shared with inbound guard, so
// one request is charged the same way in both directions.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [chain.ErrRateLimited]. By default
// the stage counts in a budget the executor holds for its service, exactly like
// the other stages hold their state — one budget per service, since one
// executor serves one service and the manager builds one executor per label.
//
// One thing goes beyond that default, and it is a [ClusterLimiter] store's
// property: HOW WIDE a budget reaches. Supply one over a shared backend (Redis,
// typically) and the limit covers every replica. With no store, the budget is
// this executor's alone.

package resilience

import (
	"context"

	"go-spring.org/cloud/runtime/dataplane/ratelimit"
)

// Algorithm names the counting strategy a rate limit asks for.
type Algorithm = ratelimit.Algorithm

const (
	// TokenBucket refills tokens continuously at [ClientPolicy.RateLimit] up to
	// [ClientPolicy.Burst]; it smooths bursts and is the default.
	TokenBucket = ratelimit.AlgorithmTokenBucket

	// SlidingWindow counts events over a rolling [ClientPolicy.Window], giving a
	// hard cap on events per window with less burst tolerance than a token
	// bucket.
	SlidingWindow = ratelimit.AlgorithmSlidingWindow
)

// RateSpec is the rate-limit stage's own vocabulary: the knobs a budget is
// built from. Both directions assemble it from their policy, so the stage — and
// any [ClusterLimiter] store behind it — never sees a direction and needs no
// second implementation for inbound. A zero RateLimit means unlimited.
type RateSpec = ratelimit.Spec

// ClusterLimiter is the rate-limit stage's cluster-wide seam: the store a rate
// limit lives in when it must hold across every replica, not just this process.
// Handing one to a driver makes the executors it builds charge that store under
// the service each executor is bound to, so a limit over a shared backend
// (Redis, typically) holds across replicas.
//
// A client never builds one: the container provides it if some backend starter
// contributed it, and the default driver passes it to the executors it builds
// (see [NewDefaultDriver]).
//
// Implementations must be safe for concurrent use and must keep each scope's
// counters apart.
type ClusterLimiter = ratelimit.ClusterLimiter

// allowRate charges one unit of the rate-limit stage for scope — an executor's
// service — and reports whether it was admitted. It spends the supplied store
// when the driver was handed one and otherwise the budget the caller holds
// (rate). A non-nil error means the store failed, which the caller treats as
// "allow" (see the executor's gate).
func allowRate(ctx context.Context, rate ratelimit.Budget, spec RateSpec, scope string, limiter ClusterLimiter) (bool, error) {
	return ratelimit.Charge(ctx, limiter, scope, spec, rate, 1)
}
