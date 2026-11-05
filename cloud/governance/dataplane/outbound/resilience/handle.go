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

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
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

// subscriber is one caller's interest in the limits of a label. last is the
// most recent limits the label resolved to, so an apply can skip notifications
// whose limits are unchanged and avoid firing on a no-op push. The notify
// callback carries no value: the subscriber re-reads the limits itself.
type subscriber struct {
	last   Limits
	notify func()
}
