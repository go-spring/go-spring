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

// ratelimit.go is the outbound view of the shared rate-limit stage: the manager
// resolves a rule's limiter KIND to a [ratelimit.Provider], and the executor
// keeps one [ratelimit.Limiter] per rate key, built by that factory. The
// counting algorithms, the [ratelimit.Policy] vocabulary and both factory
// families live in go-spring.org/cloud/governance/capability/ratelimit, a leaf
// package shared with inbound admission, so one request is charged the same way
// in both directions.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [chain.ErrRateLimited]. By default
// the kind is a process-local algorithm, so the counter lives in the executor;
// naming a shared store's kind (a Redis factory bean) makes the same rule cover
// every replica.

package resilience

import (
	"context"

	"go-spring.org/cloud/governance/capability/ratelimit"
)

// allowRate charges n units of the rate-limit stage and reports whether they
// were admitted. limiter is the counter the rule's factory built; nil means the
// stage is off. A non-nil error means the store failed, which the caller treats
// as "allow" (see the executor's gate).
func allowRate(ctx context.Context, limiter ratelimit.Limiter, policy ratelimit.Policy, n int) (bool, error) {
	if limiter == nil {
		return true, nil
	}
	return limiter.Allow(ctx, policy, n)
}
