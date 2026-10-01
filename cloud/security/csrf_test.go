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

func TestDefaultSafeMethods(t *testing.T) {
	got := DefaultSafeMethods()
	want := []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	if len(got) != len(want) {
		t.Fatalf("DefaultSafeMethods() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DefaultSafeMethods() = %v, want %v", got, want)
		}
	}
	// A fresh slice each call: mutating one must not affect the next.
	got[0] = "POST"
	if DefaultSafeMethods()[0] != "GET" {
		t.Fatal("DefaultSafeMethods must not share state between calls")
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
