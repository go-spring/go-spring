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

// observe.go declares what an InfluxDB request IS. The signals themselves —
// the span, the duration metrics, the access log — are emitted by the
// resilience layer, the one place on the executor chain that sees a whole call
// (retries included). This file therefore holds no emission code: only the
// vocabulary that this starter alone knows, because only it knows these calls
// reach an InfluxDB backend.
//
// influxdb-client-go ships no OTel instrumentation of its own, but that is no
// longer this starter's problem: the request identity declared here is read by
// the resilience layer's emitter (see [declareTransport] for why the declaration
// must run outside the executor, not inside it).

package StarterInfluxdb

import (
	"net/http"
	"strings"

	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/strutil"

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// influxdbSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const influxdbSystem = "influxdb"

// maxStatement bounds the URL path captured as db.statement. A path can carry a
// bucket or measurement and grow long; a span or a log line has no use for all
// of it.
const maxStatement = 512

// writeMethod and writePath identify the InfluxDB write endpoint — the one path
// the async writer (see [Client.ManagedWriteAPI]) posts batches to. The
// synchronous path derives the same pair from the request.
const (
	writeMethod = "POST"
	writePath   = "/api/v2/write"
)

// accessTag is the static log tag for the influxdb access log. It is registered
// here, at package init, because a tag must exist before the framework's first
// property refresh — see [log.RegisterTag].
var accessTag = log.RegisterAppTag("influxdb", "access")

// operation is the semantic identity of one InfluxDB HTTP request.
//
// The method alone rides in Attrs as db.operation: it is a bounded set
// ("post"/"get"/"delete"...) and therefore safe as a metric label. The URL path
// — which may carry the org, the bucket or a measurement, all drawn from an open
// set — rides in Detail as db.statement: it reaches the span and the log, where
// the path is exactly what makes a line worth reading, and never a label. A
// request with no path (a bare-host call) carries no detail at all, which is
// also what levels its success log at Info.
//
// The span Name keeps the request's full "METHOD /path" form, so a trace reads
// the endpoint directly even though the metric label cannot carry it.
func operation(method, path string) observability.Operation {
	o := observability.Operation{
		Name:   method + " " + path,
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", influxdbSystem),
			attribute.String("db.operation", strings.ToLower(method)),
		},
		LogTag: accessTag,
	}
	if path != "" {
		o.Detail = []attribute.KeyValue{
			attribute.String("db.statement", strutil.Truncate(path, maxStatement)),
		}
	}
	return o
}

// declareTransport is the declaration layer of the request chain: it puts the
// request's identity on the context, which the resilience layer INSIDE it reads
// to emit the span, the metrics and the access log (see [NewClient]
// for the assembly that guarantees that order).
//
// It MUST wrap the resilience round-tripper, not sit beneath it: the emitter
// reads the Operation at Execute entry, so a declaration made per attempt —
// inside the executor, on the base transport — would be read by nobody. This
// transport is therefore the outermost one, and the resilience round-tripper is
// its base.
type declareTransport struct {
	base http.RoundTripper
}

// RoundTrip stamps the request's Operation onto its context and delegates to the
// resilience round-tripper below, which reads it back and emits the call.
func (t *declareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := observability.WithOperation(req.Context(), operation(req.Method, req.URL.Path))
	return t.base.RoundTrip(req.WithContext(ctx))
}

// asyncWriteFields returns the fields an async write failure carries: the same
// db.operation / db.statement vocabulary the emitter's access log uses for the
// write endpoint, so the line joins the db.client.* records for that operation
// instead of only describing the failure in prose. The async writer's batches
// never cross the resilience executor (see [Client.ManagedWriteAPI]), so this
// failure is the one signal the single emitter cannot produce for them.
func asyncWriteFields() []log.Field {
	return []log.Field{
		log.String("db.operation", strings.ToLower(writeMethod)),
		log.String("db.statement", writePath),
		log.String("status", "error"),
	}
}
