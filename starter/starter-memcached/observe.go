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

// observe.go declares what a memcached operation IS. The signals themselves —
// the span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls reach a cache.

package StarterMemcached

import (
	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/log"
)

// memcachedSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const memcachedSystem = "memcached"

// accessTag is the static log tag for the memcached access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("memcached", "access")

// operation is the semantic identity of one memcached command. The bounded
// operation word names the command; the open-set key rides in detail, reaching
// the span and the log but never a label. An empty key (Ping, FlushAll) carries
// no detail, which levels its success log at Info. The family's vocabulary lives
// in [wire].
func operation(op, key string) chain.Operation {
	o := chain.DBOp(memcachedSystem, op, key)
	o.Observe.Log.Tag = accessTag
	return o
}
