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

// command.go is the "command seam" concept of this starter: the request-derived
// operation the resilience round-tripper reads at Execute entry to emit the one
// call span, the call-level and attempt-level duration metrics and the single
// access log.

package StarterElasticsearch

import (
	"net/http"

	"go-spring.org/cloud/chain"
)

// callFor derives a request's semantic identity — its method + URL path, see
// [operation] — so the resilience round-tripper can name the call. An
// http.RoundTripper is handed only the request, so this derivation is the
// seam's ctx-free way to declare the operation: it is passed to
// [resilience.NewRoundTripper] as the callFor callback, and every link of the
// chain reads the operation from the Execute signature rather than from the
// context. [Client.installTransport] wires it in.
func callFor(req *http.Request) chain.Operation {
	return operation(req.Method, req.URL.Path)
}
