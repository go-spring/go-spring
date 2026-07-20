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

package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
)

// bodyEqual reports whether want and got represent the same body. When both are
// valid JSON they are compared by structure (key order and whitespace ignored),
// so a contract fixture stays readable without forcing byte-exact formatting;
// otherwise the raw bytes must match. An empty want imposes no constraint.
func bodyEqual(want, got []byte) bool {
	if len(want) == 0 {
		return true
	}
	var wv, gv any
	if json.Unmarshal(want, &wv) == nil && json.Unmarshal(got, &gv) == nil {
		return reflect.DeepEqual(wv, gv)
	}
	return bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got))
}

// requestMatches reports whether an incoming request satisfies c.Request. Only
// the fields the contract sets are checked: method and path always, then any
// declared query parameters, headers, and (if present) the body. reqBody is the
// already-read request body so callers can reuse it.
func requestMatches(c Contract, r *http.Request, reqBody []byte) bool {
	if !equalFoldMethod(c.Request.Method, r.Method) || c.Request.Path != r.URL.Path {
		return false
	}
	q := r.URL.Query()
	for k, v := range c.Request.Query {
		if q.Get(k) != v {
			return false
		}
	}
	for k, v := range c.Request.Headers {
		if r.Header.Get(k) != v {
			return false
		}
	}
	return bodyEqual(c.Request.Body, reqBody)
}

// equalFoldMethod compares HTTP methods case-insensitively; an empty contract
// method matches any method so a path-only contract stays permissive.
func equalFoldMethod(want, got string) bool {
	if want == "" {
		return true
	}
	return http.CanonicalHeaderKey(want) == http.CanonicalHeaderKey(got)
}
