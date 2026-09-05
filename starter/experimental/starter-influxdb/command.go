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

// command.go is the "command seam" concept of this starter: the obsTransport
// round-tripper that emits a span + duration metric + access log per request.
// influxdb-client-go ships no OTel instrumentation of its own, so the
// transport carries the trace signal too. It mirrors starter-s3's command.go.
package StarterInfluxdb

import (
	"net/http"
)

// obsTransport wraps the underlying HTTP round-tripper so each request emits
// a span + duration metric + access log (see observe.go). The operation is
// derived from the request method + URL path (e.g. "POST /api/v2/write").
type obsTransport struct {
	base http.RoundTripper
	obs  *dbObserver
}

func (t *obsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	op := req.Method + " " + req.URL.Path
	ctx, sp := t.obs.Start(req.Context(), op, req.URL.Path)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	sp.End(err)
	return resp, err
}
