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

// observe.go declares what a cassandra operation IS. The signals themselves —
// the span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls reach a
// Cassandra backend.

package StarterCassandra

import (
	"context"

	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/log"
)

// cassandraSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const cassandraSystem = "cassandra"

// accessTag is the static log tag for the cassandra access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("cassandra", "access")

// operation is the semantic identity of one cassandra statement. op names the
// operation ("exec", "query"); stmt is the CQL text. The family's vocabulary —
// attribute keys, the bounded/open split that puts the statement in detail, the
// truncation bound — lives in [wire]; this starter supplies only what only it
// knows.
func operation(ctx context.Context, prop traffic.Propagator, op, stmt string) chain.Operation {
	o := chain.DBOp(ctx, prop, cassandraSystem, op, stmt)
	o.Observe.Log.Tag = accessTag
	return o
}
