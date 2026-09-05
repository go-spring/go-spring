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

func TestNewCSRFToken(t *testing.T) {
	a, b := NewCSRFToken(), NewCSRFToken()
	if a == "" || len(a) < 32 {
		t.Fatalf("token %q too short", a)
	}
	if a == b {
		t.Fatal("two tokens should differ")
	}
}

func TestMatchCSRFToken(t *testing.T) {
	tok := NewCSRFToken()
	if !MatchCSRFToken(tok, tok) {
		t.Fatal("matching tokens should match")
	}
	if MatchCSRFToken(tok, "wrong") {
		t.Fatal("mismatching tokens should not match")
	}
	if MatchCSRFToken("", tok) || MatchCSRFToken(tok, "") || MatchCSRFToken("", "") {
		t.Fatal("an empty side is never a match")
	}
}
