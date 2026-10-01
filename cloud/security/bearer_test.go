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

package security

import "testing"

func TestParseBearerToken(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"bearer", "Bearer abc", "abc"},
		{"case-insensitive-scheme", "bearer abc", "abc"},
		{"trims-space", "Bearer   abc  ", "abc"},
		{"basic", "Basic dXNlcjpwYXNz", ""},
		{"no-scheme", "abc", ""},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBearerToken(tt.header); got != tt.want {
				t.Fatalf("ParseBearerToken(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func TestBearerChallenge(t *testing.T) {
	// No credential at all: the bare scheme, no error code.
	if got := BearerChallenge(""); got != "Bearer" {
		t.Fatalf("BearerChallenge(\"\") = %q, want %q", got, "Bearer")
	}
	if got := BearerChallenge(BearerErrorInvalidToken); got != `Bearer error="invalid_token"` {
		t.Fatalf("BearerChallenge(invalid_token) = %q", got)
	}
	if got := BearerChallenge(BearerErrorInsufficientScope); got != `Bearer error="insufficient_scope"` {
		t.Fatalf("BearerChallenge(insufficient_scope) = %q", got)
	}
}
