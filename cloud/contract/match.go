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
	"net/url"
	"reflect"
	"slices"
	"strings"
)

// bodyEqual reports whether want and got represent the same body. contentType
// decides the comparison: a form body is compared by parsed parameter set (so
// pair order and percent-encoding differences between producers don't fail the
// match), any other body that is valid JSON on both sides is compared by
// structure (key order and whitespace ignored), and everything else must match
// by raw bytes. An empty want imposes no constraint.
func bodyEqual(want, got []byte, contentType string) bool {
	if len(want) == 0 {
		return true
	}
	if isFormContent(contentType) {
		wq, werr := url.ParseQuery(string(want))
		gq, gerr := url.ParseQuery(string(got))
		if werr == nil && gerr == nil {
			return reflect.DeepEqual(wq, gq)
		}
	}
	var wv, gv any
	if json.Unmarshal(want, &wv) == nil && json.Unmarshal(got, &gv) == nil {
		return reflect.DeepEqual(wv, gv)
	}
	return bytes.Equal(bytes.TrimSpace(want), bytes.TrimSpace(got))
}

// isFormContent reports whether contentType names the HTML form encoding, the
// one body format whose serialization is not canonical across languages (pair
// order, percent-encoding) and therefore needs parsed comparison.
func isFormContent(contentType string) bool {
	mt, _, _ := strings.Cut(contentType, ";")
	return strings.TrimSpace(strings.ToLower(mt)) == "application/x-www-form-urlencoded"
}

// valuesEqual reports whether want and got hold the same value set: same
// elements, same counts, any order — the multi-valued counterpart of a string
// equality, matching how url.Values and http.Header give no meaning to value
// order. Sorting both copies keeps it a pure comparison.
func valuesEqual(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	w := slices.Clone(want)
	g := slices.Clone(got)
	slices.Sort(w)
	slices.Sort(g)
	return slices.Equal(w, g)
}

// requestMatches reports whether an incoming request satisfies c.Request. Only
// the fields the contract sets are checked: method and path always, then any
// declared query parameters, headers, and (if present) the body. reqBody is the
// already-read request body so callers can reuse it.
func requestMatches(c Contract, r *http.Request, reqBody []byte) bool {
	// HTTP methods are case-sensitive on the wire in theory but producers are
	// sloppy in practice, so compare case-insensitively.
	if !strings.EqualFold(c.Request.Method, r.Method) || c.Request.Path != r.URL.Path {
		return false
	}
	q := r.URL.Query()
	for k, want := range c.Request.Query {
		if !valuesEqual(want, q[k]) {
			return false
		}
	}
	for k, want := range c.Request.Headers {
		if !valuesEqual(want, r.Header.Values(k)) {
			return false
		}
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		// The wire request carried no Content-Type; the contract's own declared
		// header is the next best statement of the body's format.
		ct = c.Request.Headers.Get("Content-Type")
	}
	return bodyEqual(c.Request.Body, reqBody, ct)
}
