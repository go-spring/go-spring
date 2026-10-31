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

// command.go is the "identity seam" concept of this starter: the callFor
// resolver that builds each S3 request's semantic identity (see observe.go).
// It emits nothing — the resilience round-tripper builds the operation through
// it and passes it explicitly to the executor, which emits the span, the
// metrics and the access log.

package StarterS3

import (
	"context"
	"net/http"

	"go-spring.org/cloud/governance/capability/traffic"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// callFor resolves the semantic identity of an S3 request. It is handed to
// resilience.NewRoundTripper as its operation resolver, so the round-tripper
// builds the operation at [chain.Executor.Execute] entry and passes it
// explicitly to the executor — the executor emits one call's signals covering
// every attempt, retries included. Building the operation inside the
// round-tripper (rather than on the request's context from an outer transport)
// is what keeps the identity known at the point the executor reads it.
func callFor(ctx context.Context, prop traffic.Propagator, req *http.Request) chain.Operation {
	return operation(ctx, prop, req.Method, req.URL.Path)
}
