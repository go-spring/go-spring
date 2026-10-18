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

package observe

import (
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ServerOperation is the declaration of one inbound request's signals: what the
// adapter knows before the handler runs. It is the server-side counterpart of
// the client's operation description, without the client-only vocabulary —
// there is no Kind, no System and no retry declaration here.
type ServerOperation struct {
	// Action is the interface being invoked, in the protocol's own format
	// ("GET /users/{id}", "/pkg.Svc/Method"). It names the span and is the
	// access log's identity.
	Action string

	// Metadata is the request's targeting set (headers, typically), read by
	// admission's rule matching. It is never emitted.
	Metadata map[string]string

	// Observe is the signal declaration. It is DECLARATION ONLY; the response
	// half is recorded through [Observation.Response].
	Observe ServerObserve
}

// ServerObserve groups the server's signal declarations by signal type.
type ServerObserve struct {
	Log    ServerLog
	Span   ServerSpan
	Metric ServerMetric
}

// ServerLog is the access-log group of a server operation.
type ServerLog struct {
	// Tag is the tag the access log is written under.
	Tag *log.Tag

	// QuietSuccess levels a successful request's access line at Debug.
	QuietSuccess bool

	// Fields are the request-half access-log fields, verbatim.
	Fields []log.Field
}

// ServerSpan is the trace group of a server operation.
type ServerSpan struct {
	// Attrs are the span attributes.
	Attrs []attribute.KeyValue

	// Links are the trace links this request adds (a consumer edge's producer).
	Links []trace.Link
}

// ServerMetric is the metric group of a server operation.
type ServerMetric struct {
	// Name is the metric family prefix ("http.server"). Empty means the request
	// declares no signals and takes the observer's fallback path.
	Name string

	// Attrs are the bounded metric labels.
	Attrs []attribute.KeyValue
}

// Described reports whether the request declares a metric family, which is the
// marker for "this route described itself".
func (o ServerOperation) Described() bool { return o.Observe.Metric.Name != "" }
