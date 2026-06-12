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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDirRejectsNonGenericInstantiation(t *testing.T) {
	dir := t.TempDir()
	writeParserTestProject(t, dir, `
type Page {
    required string id
}

type StringPage Page<string>
`)

	_, err := ParseDir(dir)
	if err == nil {
		t.Fatal("expected non-generic instantiation error")
	}
	if !strings.Contains(err.Error(), "type Page is not generic") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDirValidationHandlesTypeCycles(t *testing.T) {
	dir := t.TempDir()
	writeParserTestProject(t, dir, `
type NodeReq {
    Node child
}

type Node {
    NodeReq parent
    required string name
}

type NodeResp {
    required string ok
}

rpc GetNode(NodeReq) NodeResp {
    method="POST"
    path="/node"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	project, err := ParseDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	req, ok := FindType(project.Files, "NodeReq")
	if !ok {
		t.Fatal("NodeReq not found")
	}
	if !req.Type.Validate || !req.Type.Fields[0].ValidateNested {
		t.Fatalf("expected NodeReq.child to require nested validation: %#v", req.Type)
	}
	node, ok := FindType(project.Files, "Node")
	if !ok {
		t.Fatal("Node not found")
	}
	if !node.Type.Validate || !node.Type.Fields[0].ValidateNested {
		t.Fatalf("expected Node.parent to require nested validation: %#v", node.Type)
	}
}

func TestParseDirRejectsDuplicatePathParameter(t *testing.T) {
	dir := t.TempDir()
	writeParserTestProject(t, dir, `
type Req {
    required string id (path="id")
}

type Resp {
    required string ok
}

rpc GetThing(Req) Resp {
    method="GET"
    path="/parents/{id}/children/{id}"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	_, err := ParseDir(dir)
	if err == nil {
		t.Fatal("expected duplicate path parameter error")
	}
	if !strings.Contains(err.Error(), "duplicate path parameter id in rpc GetThing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDirRejectsDuplicateFieldBindings(t *testing.T) {
	tests := []struct {
		name string
		idl  string
		want string
	}{
		{
			name: "path",
			idl: `
type Req {
    required string id (path="id")
    required string other_id (path="id")
}

type Resp {
    required string ok
}

rpc GetThing(Req) Resp {
    method="GET"
    path="/things/{id}"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`,
			want: "type Req has duplicate path binding id for field id and other_id",
		},
		{
			name: "query",
			idl: `
type Req {
    string name (query="q")
    string email (query="q")
}

type Resp {
    required string ok
}

rpc Search(Req) Resp {
    method="GET"
    path="/search"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`,
			want: "type Req has duplicate query binding q for field name and email",
		},
		{
			name: "form",
			idl: `
type Req {
    string name (form="q")
    string email (form="q")
}

type Resp {
    required string ok
}

rpc Create(Req) Resp {
    method="POST"
    path="/create"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`,
			want: "type Req has duplicate form field q for field name and email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeParserTestProject(t, dir, tt.idl)
			_, err := ParseDir(dir)
			if err == nil {
				t.Fatal("expected duplicate binding error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func writeParserTestProject(t *testing.T, dir, idl string) {
	t.Helper()
	writeParserTestFile(t, dir, "meta.json", `{
  "name": "ParserTest",
  "description": "Parser test project",
  "version": "v1"
}`)
	writeParserTestFile(t, dir, "test.idl", idl)
}

func writeParserTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm); err != nil {
		t.Fatal(err)
	}
}
