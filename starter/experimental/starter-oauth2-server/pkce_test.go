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

package StarterOAuth2Server

import "testing"

func TestPKCE(t *testing.T) {
	verifier := GenerateVerifier()
	if len(verifier) < 43 {
		t.Fatalf("verifier too short: %d", len(verifier))
	}

	// S256 round trip.
	ch := Challenge(verifier, "S256")
	if ch == "" || ch == verifier {
		t.Fatalf("S256 challenge should differ from verifier, got %q", ch)
	}
	if !verifyPKCE(verifier, ch, "S256") {
		t.Fatal("S256 verifier should match its challenge")
	}
	if verifyPKCE("wrong", ch, "S256") {
		t.Fatal("wrong verifier must not match")
	}

	// plain (and empty default) is the identity.
	if Challenge(verifier, "plain") != verifier {
		t.Fatal("plain challenge must equal the verifier")
	}
	if !verifyPKCE(verifier, verifier, "plain") {
		t.Fatal("plain verifier should match")
	}
	if !verifyPKCE(verifier, verifier, "") {
		t.Fatal("empty method should behave like plain")
	}

	// Unknown method never matches.
	if verifyPKCE(verifier, ch, "bogus") {
		t.Fatal("unknown method must not verify")
	}
}
