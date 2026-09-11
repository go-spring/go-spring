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

// Package contract is the Go-Spring equivalent of Spring Cloud Contract: one
// declarative contract (a request shape paired with the response it must
// produce) drives both ends of a service-to-service call.
//
//   - On the provider side [VerifyHandler] replays every contract against the
//     real handler (or [Verify] against a live server's base URL) and asserts
//     the response matches — the provider cannot drift from the agreement
//     without a test failure.
//   - On the consumer side [StubServer] turns the same contracts into a stub
//     HTTP server that answers exactly as the provider promised, so a consumer
//     (typically a declarative HTTP client)
//     can be tested in isolation against a faithful double.
//
// Because one artifact feeds both directions, a consumer stub can never encode
// a response the provider does not actually return.
//
// Contracts are plain Go structs. On disk they are JSON so the package stays
// dependency-free; callers who prefer YAML unmarshal it themselves and hand the
// resulting []Contract to Verify / StubServer.
//
// The HTTP engine lives here because it needs nothing beyond net/http. Future
// protocol engines (gRPC, Thrift) that require protocol SDKs are expected to
// grow as sibling packages under this directory, where the cloud module's
// dependency rules allow them.
package contract

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
)

// Contract is a single agreement between a consumer and a provider: when a
// request matching Request arrives, the provider must answer with Response.
type Contract struct {
	// Name identifies the contract in diagnostics and failure messages.
	Name string `json:"name"`
	// Request is the shape a matching call must have.
	Request Request `json:"request"`
	// Response is what the provider promises to return for that request.
	Response Response `json:"response"`
}

// Values maps a name to its values — the url.Values / http.Header shape, so
// multi-valued names (?tag=a&tag=b, repeated Set-Cookie) are modelled the same
// way the standard library models them. In JSON each name accepts either a
// single string or an array of strings:
//
//	{"name": "Ada"}      // one value
//	{"tag": ["a", "b"]}  // several values
//
// A single string desugars to a one-element slice, so both spellings mean the
// same thing on the Go side.
type Values map[string][]string

// UnmarshalJSON accepts, per name, either a JSON string or an array of JSON
// strings; anything else is an error.
func (v *Values) UnmarshalJSON(b []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	out := make(Values, len(raw))
	for k, rv := range raw {
		var one string
		if err := json.Unmarshal(rv, &one); err == nil {
			out[k] = []string{one}
			continue
		}
		var many []string
		if err := json.Unmarshal(rv, &many); err != nil {
			return fmt.Errorf("value of %q must be a string or an array of strings", k)
		}
		out[k] = many
	}
	*v = out
	return nil
}

// Get returns the first value of name, or "" — the url.Values.Get /
// http.Header.Get convention.
func (v Values) Get(name string) string {
	if vs := v[name]; len(vs) > 0 {
		return vs[0]
	}
	return ""
}

// Request is the request side of a contract. Only the fields that are set take
// part in matching: an empty Query/Headers map imposes no constraint, and a nil
// Body means the body is not inspected. This lets a contract pin just the parts
// that matter to the agreement.
type Request struct {
	// Method is the HTTP method (GET, POST, ...). Required.
	Method string `json:"method"`
	// Path is the request path, e.g. "/greet". Required.
	Path string `json:"path"`
	// Query lists query parameters that must be present: for each name, the
	// request must carry exactly the listed values — same set, any order, same
	// count. Names not listed are unconstrained.
	Query Values `json:"query,omitempty"`
	// Headers lists request headers under the same exact-match rule as Query.
	Headers Values `json:"headers,omitempty"`
	// Body, when non-nil, must match the incoming body. JSON bodies compare by
	// structure (key order and formatting ignored); form bodies (per the
	// Content-Type) compare by parsed parameter set (pair order and encoding
	// ignored); anything else compares by raw bytes.
	Body json.RawMessage `json:"body,omitempty"`
}

// Response is the response side of a contract: what the stub replays and what
// the provider is verified against.
type Response struct {
	// Status is the HTTP status code. Required; it must be in 100-599.
	Status int `json:"status"`
	// Headers are response headers to set (stub) or assert (verify), under the
	// same exact-match rule as Request.Query.
	Headers Values `json:"headers,omitempty"`
	// Body is the response body. JSON bodies compare by structural equality,
	// form bodies (per the Content-Type) by parsed parameter set, anything else
	// by raw bytes.
	Body json.RawMessage `json:"body,omitempty"`
}

// Load reads one or more JSON files and returns the contracts they contain.
// Each file holds either a single Contract object or a JSON array of them, so a
// suite can be split across files or kept in one. It fails fast on a missing
// file or malformed JSON so a broken contract surfaces before any test runs.
func Load(paths ...string) ([]Contract, error) {
	var out []Contract
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		cs, err := decode(data)
		if err != nil {
			return nil, fmt.Errorf("contract: parse %s: %w", p, err)
		}
		out = append(out, cs...)
	}
	return out, nil
}

// LoadFS is like [Load] but reads every file matching glob from fsys, which is
// handy with an embed.FS of contract fixtures shipped alongside a test.
func LoadFS(fsys fs.FS, pattern string) ([]Contract, error) {
	matches, err := fs.Glob(fsys, pattern)
	if err != nil {
		return nil, err
	}
	var out []Contract
	for _, m := range matches {
		data, err := fs.ReadFile(fsys, m)
		if err != nil {
			return nil, err
		}
		cs, err := decode(data)
		if err != nil {
			return nil, fmt.Errorf("contract: parse %s: %w", m, err)
		}
		out = append(out, cs...)
	}
	return out, nil
}

// decode parses a file body as either a single contract or an array of them,
// deciding by the first non-space byte so both on-disk layouts are accepted.
func decode(data []byte) ([]Contract, error) {
	if data = bytes.TrimSpace(data); len(data) == 0 {
		return nil, nil
	}
	if data[0] == '[' {
		var cs []Contract
		if err := json.Unmarshal(data, &cs); err != nil {
			return nil, err
		}
		for i := range cs {
			if err := cs[i].validate(); err != nil {
				return nil, err
			}
		}
		return cs, nil
	}
	var c Contract
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if err := c.validate(); err != nil {
		return nil, err
	}
	return []Contract{c}, nil
}

// validate fails fast on a contract missing its required fields, so a broken
// file surfaces at Load time with a pointed message instead of as an
// indirect "no contract matched" at stub or verify time.
func (c *Contract) validate() error {
	if c.Request.Method == "" {
		return fmt.Errorf("contract %q: request.method is required", c.Name)
	}
	if c.Request.Path == "" {
		return fmt.Errorf("contract %q: request.path is required", c.Name)
	}
	if c.Response.Status < 100 || c.Response.Status > 599 {
		return fmt.Errorf("contract %q: response.status %d is missing or not a valid HTTP status code", c.Name, c.Response.Status)
	}
	return nil
}
