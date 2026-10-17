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
// the resilience layer's emitter (see [callFor] for how the identity reaches the
// executor — through the callback it derives the operation with, not the
// context).

package StarterInfluxdb

import (
	"net/http"
	"strings"

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
)

// influxdbSystem is the value the family's db.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice.
const influxdbSystem = "influxdb"

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

// operation is the semantic identity of one InfluxDB HTTP request. The method —
// lowercased, the family's convention — is the bounded operation word; the
// open-set path rides in detail, while the span Name keeps the request's full
// "METHOD /path" form. The family's vocabulary lives in [wire].
func operation(method, path string) chain.Operation {
	o := chain.DBOp(influxdbSystem, strings.ToLower(method), path)
	o.Request.Action = method + " " + path
	o.Observe.Log.Tag = accessTag
	return o
}

// callFor derives a request's semantic identity — its method + URL path, see
// [operation] — so the resilience round-tripper can name the call. An
// http.RoundTripper is handed only the request, so this derivation is the
// seam's ctx-free way to declare the operation: it is passed to
// [resilience.NewRoundTripper] as the callFor callback, and the executor reads
// the operation from the Execute signature rather than from the context. Every
// link of the chain therefore sees the whole call from Execute entry, attempts
// included. [NewClient] wires it in.
func callFor(req *http.Request) chain.Operation {
	return operation(req.Method, req.URL.Path)
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
