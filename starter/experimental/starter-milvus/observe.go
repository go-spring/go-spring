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

// observe.go declares what a Milvus operation IS. The signals themselves — the
// span, the duration metrics, the access log — are emitted by the resilience
// layer, the one place on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these calls reach a
// Milvus cluster.

package StarterMilvus

import (
	"strings"

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// milvusSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const milvusSystem = "milvus"

// accessTag is the static log tag for the Milvus access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterAppTag].
var accessTag = log.RegisterAppTag("milvus", "access")

// operation is the semantic identity of one Milvus RPC, derived from the gRPC
// method the guard interceptor is handed (guard.go). The Milvus SDK is
// gRPC-based, so the only thing the transport exposes at the guard is the fully
// qualified method path, e.g. "/milvus.proto.milvus.MilvusService/Search".
//
// The operation name is the whole method's last segment ("Search") — a bounded
// word drawn from the SDK's fixed method set. The collection name and the search
// expression are NOT available here (caller-supplied arguments, consumed inside
// the SDK), so nothing unbounded is ever put at risk of becoming a label. The
// full method path rides in detail under the family's own key, reaching the span
// and the log but never a label. The family's vocabulary lives in [wire].
func operation(method string) chain.Operation {
	name := method
	if i := strings.LastIndex(method, "/"); i >= 0 {
		name = method[i+1:]
	}
	o := chain.DBOp(milvusSystem, name, "")
	o.Observe.Span.Attrs = append(o.Observe.Span.Attrs, attribute.String("db.method", method))
	o.Observe.Log.Fields = append(o.Observe.Log.Fields, log.String("db.method", method))
	o.Observe.Log.QuietSuccess = true
	o.Observe.Log.Tag = accessTag
	return o
}
