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

package source

import (
	"testing"
	"time"
)

// testDoc is a stand-in document for the transport tests. It has the two shapes
// the transport must handle: a scalar and a list of structs (slices and maps are
// why dedupe uses reflect.DeepEqual).
type testDoc struct {
	Client testClient `value:"${spring.test.client:=}"`
}

type testClient struct {
	Timeout time.Duration `value:"${timeout:=0}"`
	Rules   []testRule    `value:"${rules:=}"`
}

type testRule struct {
	Name string  `value:"${name:=}"`
	Rate float64 `value:"${rate:=0}"`
}

// TestParse_BindsWholeDocument pins that a document binds by its top-level keys
// and that a foreign or empty document binds the zero value instead of failing —
// there is no root-key guard; a root that cannot be disarmed enforces that in its
// own binding.
func TestParse_BindsWholeDocument(t *testing.T) {
	const doc = "spring.test.client.timeout=100ms\n"
	got, err := Parse[testDoc]("doc.properties", []byte(doc), "")
	if err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
	if got.Client.Timeout != 100*time.Millisecond {
		t.Fatalf("timeout = %v, want 100ms", got.Client.Timeout)
	}

	// The whole document is bound: keys T does not name are ignored, so a
	// document for another root binds the zero value instead of erroring. The
	// source vouches for what it pushes; a root that must not be disarmed by an
	// empty document enforces that in its own binding.
	foreign, err := Parse[testDoc]("doc.properties", []byte("spring.other.x=1\n"), "")
	if err != nil {
		t.Fatalf("foreign document rejected: %v", err)
	}
	if foreign.Client.Timeout != 0 {
		t.Fatalf("foreign document bound something: %+v", foreign)
	}

	empty, err := Parse[testDoc]("doc.properties", nil, "")
	if err != nil {
		t.Fatalf("empty document rejected: %v", err)
	}
	if empty.Client.Timeout != 0 {
		t.Fatalf("empty document bound something: %+v", empty)
	}
}

// TestParse_FormatInference pins both halves of format selection: the extension
// of name when no format is given, and an explicit format overriding it.
func TestParse_FormatInference(t *testing.T) {
	const yaml = "spring:\n  test:\n    client:\n      timeout: 250ms\n"
	got, err := Parse[testDoc]("doc.yaml", []byte(yaml), "")
	if err != nil {
		t.Fatalf("yaml rejected: %v", err)
	}
	if got.Client.Timeout != 250*time.Millisecond {
		t.Fatalf("timeout = %v, want 250ms", got.Client.Timeout)
	}

	// An explicit format overrides the name's extension.
	if _, err = Parse[testDoc]("doc.properties", []byte(yaml), "yaml"); err != nil {
		t.Fatalf("explicit format rejected: %v", err)
	}
}

// TestParse_EmptyDocumentSentinel pins the "off" spelling: a root is disarmed by
// publishing a document that configures nothing but still carries a key under
// its prefix. For a list field the empty value is "" — NOT []: flatten renders a
// non-nil empty slice as "[]", but the binder's single-string branch splits that
// into one element, so `rules: []` would bind one zero rule instead of none.
func TestParse_EmptyDocumentSentinel(t *testing.T) {
	const yaml = "spring:\n  test:\n    client:\n      rules: \"\"\n"
	got, err := Parse[testDoc]("doc.yaml", []byte(yaml), "")
	if err != nil {
		t.Fatalf("a document that configures nothing must still parse: %v", err)
	}
	if len(got.Client.Rules) != 0 {
		t.Fatalf("rules = %v, want empty", got.Client.Rules)
	}

	// The same sentinel in properties spelling.
	got, err = Parse[testDoc]("doc.properties", []byte("spring.test.client.rules=\n"), "")
	if err != nil {
		t.Fatalf("properties sentinel rejected: %v", err)
	}
	if len(got.Client.Rules) != 0 {
		t.Fatalf("rules = %v, want empty", got.Client.Rules)
	}
}
