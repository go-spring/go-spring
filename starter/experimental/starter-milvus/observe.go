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

	"go-spring.org/cloud/observability"

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
// word drawn from the SDK's fixed method set, which is what [observability.Operation.Attrs]
// demands: every attr here becomes a metric label, and the method name is the
// finest bounded dimension the transport can see. The collection name and the
// search expression are NOT available here (they are caller-supplied arguments,
// consumed inside the SDK, never reaching an interceptor) — so nothing
// unbounded is ever put at risk of becoming a label.
//
// The full method path rides in Detail rather than Attrs: it is redundant with
// the operation name for the label's purpose, yet exactly what makes a span or
// a log line unambiguous to read. Detail reaches the span and the log and never
// a label. A path-less method (the whole string, when it carries no "/") still
// declares the path as detail, so a success log stays at Debug — the frequent,
// uninteresting case — matching the other DB clients, which level their success
// lines by the presence of per-call detail.
func operation(method string) observability.Operation {
	name := method
	if i := strings.LastIndex(method, "/"); i >= 0 {
		name = method[i+1:]
	}
	return observability.Operation{
		Name:   name,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", milvusSystem),
			attribute.String("db.operation", name),
		},
		Detail: []attribute.KeyValue{
			attribute.String("db.method", method),
		},
		LogTag: accessTag,
	}
}
