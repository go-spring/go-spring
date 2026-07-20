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
//   - On the provider side [Verify] replays every contract against the real
//     handler and asserts the response matches — the provider cannot drift from
//     the agreement without a test failure.
//   - On the consumer side [StubServer] turns the same contracts into a stub
//     HTTP server that answers exactly as the provider promised, so a consumer
//     (typically a Task 01 declarative HTTP client — see go-spring.org/spring/httpx)
//     can be tested in isolation against a faithful double.
//
// Because one artifact feeds both directions, a consumer stub can never encode
// a response the provider does not actually return.
//
// Contracts are plain Go structs. On disk they are JSON so the package keeps
// stdlib's zero-dependency rule (mirroring go-spring.org/spring/i18n, which also
// declines a YAML dependency and takes already-parsed input); callers who prefer
// YAML unmarshal it themselves and hand the resulting []Contract to Verify /
// StubServer.
package contract

import (
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

// Request is the request side of a contract. Only the fields that are set take
// part in matching: an empty Query/Headers map imposes no constraint, and a nil
// Body means the body is not inspected. This lets a contract pin just the parts
// that matter to the agreement.
type Request struct {
	// Method is the HTTP method (GET, POST, ...). Required.
	Method string `json:"method"`
	// Path is the request path, e.g. "/greet". Required.
	Path string `json:"path"`
	// Query lists query parameters that must be present with these exact values.
	Query map[string]string `json:"query,omitempty"`
	// Headers lists request headers that must be present with these exact values.
	Headers map[string]string `json:"headers,omitempty"`
	// Body, when non-nil, must match the incoming body by JSON structural
	// equality (key order and formatting are ignored).
	Body json.RawMessage `json:"body,omitempty"`
}

// Response is the response side of a contract: what the stub replays and what
// the provider is verified against.
type Response struct {
	// Status is the HTTP status code. Defaults to 200 when zero.
	Status int `json:"status"`
	// Headers are response headers to set (stub) or assert (verify).
	Headers map[string]string `json:"headers,omitempty"`
	// Body is the response body. When it is valid JSON, verification compares by
	// JSON structural equality; otherwise it compares raw bytes.
	Body json.RawMessage `json:"body,omitempty"`
}

// status returns the effective status code, defaulting to 200.
func (r Response) status() int {
	if r.Status == 0 {
		return 200
	}
	return r.Status
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
func LoadFS(fsys fs.FS, glob string) ([]Contract, error) {
	matches, err := fs.Glob(fsys, glob)
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
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		case '[':
			var cs []Contract
			if err := json.Unmarshal(data, &cs); err != nil {
				return nil, err
			}
			return cs, nil
		default:
			var c Contract
			if err := json.Unmarshal(data, &c); err != nil {
				return nil, err
			}
			return []Contract{c}, nil
		}
	}
	return nil, nil
}
