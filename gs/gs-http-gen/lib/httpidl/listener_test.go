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

package httpidl

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/go-spring/stdlib/jsonflow"
)

func TestListener(t *testing.T) {
	fileName := "testdata/success/http.idl"
	b, err := os.ReadFile(fileName)
	if err != nil {
		t.Fatal(err)
	}
	doc, _, err := ParseIDL(b)
	if err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile("testdata/success/http.formated.idl")
	if err != nil {
		t.Fatal(err)
	}
	s := Format(doc)
	if s != string(b) {
		t.Fatalf("expected:\n%s\nbut got:\n%s", string(b), s)
	}
	b, err = os.ReadFile("testdata/success/http.idl.json")
	if err != nil {
		t.Fatal(err)
	}
	v, err := jsonflow.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.TrimSpace(b)
	if !bytes.Equal(v, b) {
		t.Fatalf("expected:\n%s\nbut got:\n%s", string(b), string(v))
	}
}

func TestParseIDLRejectsDuplicateTopLevelName(t *testing.T) {
	_, _, err := ParseIDL([]byte(`
type User {
    required string name
}

type User {
    required string id
}
`))
	if err == nil {
		t.Fatal("expected duplicate top-level name error")
	}
	if !strings.Contains(err.Error(), "duplicate type name User") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseIDLRejectsDuplicateRPCName(t *testing.T) {
	_, _, err := ParseIDL([]byte(`
type Req {
    required string id
}

type Resp {
    required string ok
}

rpc Save(Req) Resp {
    method="POST"
    path="/save"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}

rpc Save(Req) Resp {
    method="PUT"
    path="/save"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`))
	if err == nil {
		t.Fatal("expected duplicate rpc name error")
	}
	if !strings.Contains(err.Error(), "duplicate rpc name Save") {
		t.Fatalf("unexpected error: %v", err)
	}
}
