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

// command.go is the "declaration seam" concept of this starter: the
// declareTransport round-tripper that puts each S3 request's semantic identity
// on the context (see observe.go). It emits nothing — the resilience layer
// inside it reads the declaration to emit the span, the metrics and the access
// log.

package StarterS3

import (
	"net/http"

	"go-spring.org/cloud/observability"
)

// declareTransport declares the semantic identity of the S3 request on its
// context and delegates inward. It MUST sit OUTSIDE the resilience
// round-tripper: the emitter reads the operation at [chain.Executor.Execute]
// entry, so a declaration made inside the executor — per attempt — would be
// read by nobody. Placed here, the executor emits one call's signals covering
// every attempt, retries included.
type declareTransport struct {
	next http.RoundTripper
}

func (t *declareTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	op := operation(req.Method, req.URL.Path)
	return t.next.RoundTrip(req.WithContext(observability.WithOperation(req.Context(), op)))
}
