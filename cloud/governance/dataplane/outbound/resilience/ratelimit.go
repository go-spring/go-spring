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
// executor's own budget ([go-spring.org/cloud/governance/capability/ratelimit.Budget]) and the
// [ratelimit.ClusterLimiter] seam (where a cross-replica budget lives). The counting
// algorithms, the [ratelimit.Spec] vocabulary and the local budget live in
// go-spring.org/cloud/governance/capability/ratelimit, a leaf package shared with inbound admission, so
// one request is charged the same way in both directions.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [chain.ErrRateLimited]. By default
// the stage counts in a budget the executor holds for its service, exactly like
// the other stages hold their state — one budget per service, since one
// executor serves one service and the manager builds one executor per label.
//
// One thing goes beyond that default, and it is a [ratelimit.ClusterLimiter] store's
// property: HOW WIDE a budget reaches. Supply one over a shared backend (Redis,
// typically) and the limit covers every replica. With no store, the budget is
// this executor's alone.

package resilience

import (
	"context"

	"go-spring.org/cloud/governance/capability/ratelimit"
)

// allowRate charges one unit of the rate-limit stage for scope — an executor's
// service — and reports whether it was admitted. It spends the supplied store
// when the driver was handed one and otherwise the budget the caller holds
// (rate). A non-nil error means the store failed, which the caller treats as
// "allow" (see the executor's gate).
func allowRate(ctx context.Context, rate ratelimit.Budget, spec ratelimit.Spec, scope string, limiter ratelimit.ClusterLimiter) (bool, error) {
	return ratelimit.Charge(ctx, limiter, scope, spec, rate, 1)
}
