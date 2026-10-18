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
	"sync"

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
)

// entry is one label's outbound registration: the executor built for it —
// already instrumented. It carries no policy: the executor resolves its own, per
// call, from the manager it was handed. Entries are held by POINTER so the hot
// path never copies one merely to reach the executor inside.
type entry struct {
	exec chain.Executor
}

// drainCache empties the outbound registry and returns what was in it.
func drainCache(c *sync.Map) []chain.Executor {
	var out []chain.Executor
	c.Range(func(key, value any) bool {
		out = append(out, value.(*entry).exec)
		c.Delete(key)
		return true
	})
	return out
}

// subscriber is one caller's interest in the policy for a label. last is the most
// recent policy delivered to cb, so an apply can skip callbacks whose policy is
// unchanged and avoid re-delivering the same value.
type subscriber struct {
	last Policy
	cb   func(Policy)

	// cancelled marks a subscriber whose [Subscription.Cancel] has run. The
	// manager drops it from subs on cancel, but an Apply already in flight may
	// hold a reference to it from before the removal — so delivery checks this
	// flag under mu and skips it, making cancel take effect immediately rather
	// than after the next apply.
	cancelled bool
}

// pending is one notification collected under the manager lock and delivered
// after it is released.
type pending struct {
	cb     func(Policy)
	policy Policy
}

// Subscription is a live registration created by [Manager.Subscribe]. It carries
// the policy the callback was armed with and the ability to detach it again,
// which matters for callers whose protected objects are not process-lifetime: a
// gateway rebuilding its route table, or any client re-created on a config change.
// Without Cancel, every rebuild would leave a subscriber (and a reference to the
// discarded object) behind in the client manager forever.
//
// The zero value is inert: Policy is a zero pass-through policy and Cancel is a
// no-op, so a Subscription is always safe to hold and always safe to cancel —
// including more than once.
type Subscription struct {
	// Policy is the policy the callback was armed with at registration time.
	Policy Policy

	cancel func()
}

// Cancel detaches the subscription. It is idempotent and safe to call
// concurrently with an apply: once it returns, the callback is no longer invoked.
// Cancelling the zero Subscription is a no-op.
func (s Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}
