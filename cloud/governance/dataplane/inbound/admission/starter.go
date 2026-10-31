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

package admission

import (
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/spring/gs"
)

// The inbound admission authority is registered here, by the package that owns
// it, exactly like every other authority the runtime holds: linking this package
// is what puts the one process-wide Manager in the container, and the runtime
// composition root injects it as a field instead of constructing it.
//
// The cluster limiter is OPTIONAL ("?"): only a rate-limit backend starter (a
// Redis store, say) contributes one, and without it each Guard charges its
// per-instance budget. With one, the limit spans every replica of the service.
//
// The Manager owns its own teardown — [Manager.Close] drops the Guards and their
// stage state — so the bean declares it as the Destroy hook and the runtime does
// not close it.
func init() {
	gs.Provide(func(l ratelimit.ClusterLimiter) *Manager {
		var opts []Option
		if l != nil {
			opts = append(opts, WithClusterLimiter(l))
		}
		return NewManager(opts...)
	}, gs.IndexArg(0, gs.TagArg("?"))).
		Destroy((*Manager).Close).Caller(1)
}
