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

// observe.go declares what an Elasticsearch request IS. The signals themselves —
// the span, the call-level and attempt-level duration metrics, the in-flight
// gauge, the access log — are emitted by the resilience layer, the single point
// on the transport chain that sees a whole call (retries included). This file
// therefore holds no emission code: only the vocabulary this starter alone
// knows, because only it knows these requests reach an Elasticsearch cluster.

package StarterElasticsearch

import (
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// elasticsearchSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const elasticsearchSystem = "elasticsearch"

// maxArg bounds the URL path captured as db.statement. A path carries the index
// name and can be long; a span or a log line has no use for all of it.
const maxArg = 512

// accessTag is the static log tag for the elasticsearch access log. It is
// registered here, at package init, because a tag must exist before the
// framework's first property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("elasticsearch", "access")

// operation is the semantic identity of one Elasticsearch request, seen at the
// HTTP transport as its method and URL path.
//
// The path rides in Detail rather than Attrs: it carries the index name, drawn
// from an open set, so as a metric label it would multiply the series without
// bound. Detail reaches the span and the log — where a path is exactly what
// makes a line worth reading — and never a label. What is bounded is the HTTP
// method, which rides in db.operation as the classifiable part; the full
// "METHOD /path" names the span, which is not a label either.
func operation(method, path string) observability.Operation {
	op := observability.Operation{
		Name:   method + " " + path,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", elasticsearchSystem),
			attribute.String("db.operation", method),
		},
		LogTag: accessTag,
	}
	if path != "" {
		op.Detail = []attribute.KeyValue{
			attribute.String("db.statement", strutil.Truncate(path, maxArg)),
		}
	}
	return op
}
