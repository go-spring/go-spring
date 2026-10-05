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

// observe.go declares what one S3 call IS. The signals themselves — the span,
// the duration metrics, the access log — are emitted by the resilience layer,
// the single point on the executor chain that sees a whole call (retries
// included). This file therefore holds no emission code: only the vocabulary
// that this starter alone knows, because only it knows these requests reach an
// S3-compatible object store over HTTP.

package StarterS3

import (
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// s3System is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const s3System = "s3"

// maxStatement bounds the request path captured as db.statement. An object key
// can be long and a span or a log line has no use for all of it.
const maxStatement = 512

// accessTag is the static log tag for the s3 access log. It is registered here,
// at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("s3", "access")

// operation is the semantic identity of one S3 HTTP call.
//
// db.operation carries only the HTTP method — a bounded set — so the metric
// labels stay bounded. The URL path is drawn from an open set (bucket/key),
// so as a label it would multiply the series without bound; it therefore rides
// in Detail as db.statement, reaching the span and the log but never a label.
// The span Name keeps the descriptive "METHOD /path" form: Name is a span
// name, not a label, so it is the right place for a path to survive.
func operation(method, path string) observability.Operation {
	op := observability.Operation{
		Name:   method + " " + path,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", s3System),
			attribute.String("db.operation", method),
		},
		LogTag: accessTag,
	}
	if path != "" {
		op.Detail = []attribute.KeyValue{
			attribute.String("db.statement", strutil.Truncate(path, maxStatement)),
		}
	}
	return op
}
