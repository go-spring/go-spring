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

// observe.go declares what a TDengine statement IS. The signals themselves —
// the span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these statements reach a
// TDengine backend.

package StarterTdengine

import (
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/log"
)

// tdengineSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const tdengineSystem = "tdengine"

// accessTag is the static log tag for the tdengine access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("tdengine", "access")

// operation is the semantic identity of one TDengine statement. op is the
// statement kind — "exec" or "query" — a closed set; stmt is the SQL text. The
// family's vocabulary lives in [wire]; this starter supplies only what only it
// knows.
func operation(op, stmt string) chain.Operation {
	o := chain.DBOp(tdengineSystem, op, stmt)
	o.Observe.Log.Tag = accessTag
	return o
}
