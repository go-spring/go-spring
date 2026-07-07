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
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
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
	require.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains("type Page is not generic")
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
	require.Error(t, err).Nil()
	req, ok := FindType(project.Files, "NodeReq")
	require.That(t, ok).True()
	assert.That(t, req.Type.Validate).True()
	assert.That(t, req.Type.Fields[0].ValidateNested).True()
	node, ok := FindType(project.Files, "Node")
	require.That(t, ok).True()
	assert.That(t, node.Type.Validate).True()
	assert.That(t, node.Type.Fields[0].ValidateNested).True()
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
	require.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains("duplicate path parameter id in rpc GetThing")
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
			require.Error(t, err).NotNil()
			assert.String(t, err.Error()).Contains(tt.want)
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
	require.Error(t, os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm)).Nil()
}
