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

// command.go is the "command seam" concept of this starter: the declaration
// transport — the outer http.RoundTripper that puts each request's semantic
// identity on the context, for the resilience layer inside it to emit.
package StarterElasticsearch

import (
	"net/http"

	"go-spring.org/cloud/observability"
)

// declareTransport is the declaration layer of the transport chain: it turns the
// request's method + URL path into an [observability.Operation] and puts it on
// the request context, where the resilience executor — which reads the operation
// at Execute entry — picks it up to emit the one call span, the call-level and
// attempt-level duration metrics and the single access log.
//
// It MUST sit OUTSIDE the resilience round-tripper, not be its base: a
// declaration made inside the executor is read by nobody, because the executor
// already read the operation before the protected call ran. [Client.installTransport]
// builds them in that order.
type declareTransport struct {
	base http.RoundTripper
}

func (t *declareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := observability.WithOperation(req.Context(), operation(req.Method, req.URL.Path))
	return t.base.RoundTrip(req.WithContext(ctx))
}
