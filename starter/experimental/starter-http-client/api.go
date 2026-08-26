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

// api.go is the imperative (RestTemplate-style) half of the HTTP client: thin
// generic helpers that build an httpclt.Metadata and dispatch through
// httpclt.JSONResponse, so a one-off call needs no generated client. Because
// they dispatch through the same httpclt.DoRequest seam, every imperative call
// rides the process-wide transport this starter installs — discovery, load
// balancing, resilience and tracing apply exactly as they do for generated
// declarative clients; the Target is the same route key (addr or service name).
package StarterHTTPClient

import (
	"context"
	"net/http"
	"net/url"

	"go-spring.org/stdlib/httpclt"
)

// Get issues a GET to target/path and decodes the JSON body into T. target is
// the route key of a configured spring.http-client entry (addr or service
// name); path is the absolute request path. opts attach headers/config/query
// exactly as for a generated client.
func Get[T any](ctx context.Context, target, path string, opts ...httpclt.RequestOption) (*http.Response, T, error) {
	return Call[T](ctx, http.MethodGet, target, path, nil, opts...)
}

// Post issues a POST with a JSON-encoded body and decodes the JSON response
// into T. See [Get] for target/path semantics.
func Post[T any](ctx context.Context, target, path string, body any, opts ...httpclt.RequestOption) (*http.Response, T, error) {
	return Call[T](ctx, http.MethodPost, target, path, body, opts...)
}

// Put issues a PUT with a JSON-encoded body and decodes the JSON response
// into T. See [Get] for target/path semantics.
func Put[T any](ctx context.Context, target, path string, body any, opts ...httpclt.RequestOption) (*http.Response, T, error) {
	return Call[T](ctx, http.MethodPut, target, path, body, opts...)
}

// Delete issues a DELETE and decodes the JSON response into T. See [Get] for
// target/path semantics.
func Delete[T any](ctx context.Context, target, path string, opts ...httpclt.RequestOption) (*http.Response, T, error) {
	return Call[T](ctx, http.MethodDelete, target, path, nil, opts...)
}

// Call is the fully-explicit form behind Get/Post/Put/Delete: one request with
// an arbitrary method and body, decoded into T. schema defaults to "http" —
// pass "https" for a TLS route.
func Call[T any](ctx context.Context, method, target, path string, body any, opts ...httpclt.RequestOption) (*http.Response, T, error) {
	meta := httpclt.Metadata{
		Target:  target,
		Schema:  "http",
		Method:  method,
		RawPath: path,
		Body:    body,
	}
	return httpclt.JSONResponse[T](ctx, httpclt.CombineMetadata(meta, opts...))
}

// WithQuery attaches a URL query to the request (the raw, already-encoded
// form, e.g. "page=2&size=20"). It is the imperative counterpart of the
// QueryStringer a generated client binds.
func WithQuery(rawQuery string) httpclt.RequestOption {
	return func(meta *httpclt.Metadata) {
		meta.Query = rawQueryStringer(rawQuery)
	}
}

// WithScheme overrides the request schema (default "http"); "https" selects a
// TLS route without rebuilding the whole Metadata by hand.
func WithScheme(schema string) httpclt.RequestOption {
	return func(meta *httpclt.Metadata) {
		if schema != "" {
			meta.Schema = schema
		}
	}
}

// rawQueryStringer adapts an already-encoded query string to the
// httpclt.QueryStringer seam.
type rawQueryStringer string

func (s rawQueryStringer) QueryForm() (string, error) {
	if _, err := url.ParseQuery(string(s)); err != nil {
		return "", err
	}
	return string(s), nil
}
