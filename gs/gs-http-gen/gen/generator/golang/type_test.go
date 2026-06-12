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

package golang

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go-spring.org/gs-http-gen/lib/validate"
)

func TestGenDecodeJSONMapKey(t *testing.T) {
	got := genDecodeJSON("map[int64]string", []TypeKind{
		TypeKindMap,
		TypeKindInt,
		TypeKindString,
	})
	want := "jsonflow.DecodeMap(jsonflow.DecodeIntKey[int64], jsonflow.DecodeString)"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGenDecodeJSONAny(t *testing.T) {
	got := genDecodeJSON("any", []TypeKind{TypeKindAny})
	want := "jsonflow.DecodeAny[any]"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGenEncodeJSONMap(t *testing.T) {
	got := genEncodeJSON("map[string][]int64", []TypeKind{
		TypeKindMap,
		TypeKindString,
		TypeKindList,
		TypeKindInt,
	})
	want := "jsonflow.EncodeMap(jsonflow.EncodeStringKey[string], jsonflow.EncodeArray(jsonflow.EncodeInt[int64]))"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGenDecodeJSONRequiredCheckBytes(t *testing.T) {
	got := genDecodeJSONRequiredCheck("r.Avatar", []TypeKind{TypeKindBytes}, "avatar")
	want := `if r.Avatar == nil {
			return errutil.Explain(nil, "field \"avatar\" must not be null")
		}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCompileValidateExprQuotesSingleQuotedString(t *testing.T) {
	tests := []struct {
		name string
		expr string
		want string
	}{
		{
			name: "double_quote",
			expr: `$ == 'hello "world"'`,
			want: `x.Name == "hello \"world\""`,
		},
		{
			name: "escaped_single_quote",
			expr: `$ == 'it\'s'`,
			want: `x.Name == "it's"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := validate.Parse(tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, err := compileValidateExpr("x.Name", "string", expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenValidateNestedMapValue(t *testing.T) {
	got := genValidateNested("Req", "Contacts", "x.Contacts", []TypeKind{
		TypeKindMap,
		TypeKindString,
		TypeKindStructPtr,
	}, 0)
	if !strings.Contains(got, "for _, v0 := range x.Contacts") {
		t.Fatalf("missing map value loop: %q", got)
	}
	if !strings.Contains(got, "v0.Validate()") {
		t.Fatalf("missing map value validation: %q", got)
	}
}

func TestTypeTemplateEmitsValidateForRequiredOnlyNestedType(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
type ParentReq {
    Child child
}

type Child {
    required string name
}

type Resp {
    required string ok
}

rpc CreateParent(ParentReq) Resp {
    method="POST"
    path="/parents"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	spec, err := Convert(dir)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = typeTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Consts":  spec.Consts["test.idl"],
		"Enums":   spec.Enums["test.idl"],
		"Structs": spec.Types["test.idl"],
	})
	if err != nil {
		t.Fatal(err)
	}
	src := buf.String()
	if !strings.Contains(src, "func (x *Child) Validate() error") {
		t.Fatalf("required-only nested type should have Validate method:\n%s", src)
	}
	if !strings.Contains(src, "x.Child.Validate()") {
		t.Fatalf("parent request body should validate nested child:\n%s", src)
	}
}

func TestClientTemplateOrdersPathParamsByPathSegment(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
type RepoReq {
    required string org_id (path="orgId")
    required string repo_id (path="repoId")
}

type Resp {
    required string ok
}

rpc GetRepo(RepoReq) Resp {
    method="GET"
    path="/org/{orgId}/repos/{repoId}"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	spec, err := Convert(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(spec.RPCs) != 1 {
		t.Fatalf("expected one RPC, got %d", len(spec.RPCs))
	}
	gotFields := strings.Join(spec.RPCs[0].PathParamFields, ",")
	if gotFields != "OrgId,RepoId" {
		t.Fatalf("unexpected path param field order: %s", gotFields)
	}

	var buf bytes.Buffer
	err = clientTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"RPCs":    spec.RPCs,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `RawPath: fmt.Sprintf("/org/%v/repos/%v", req.OrgId, req.RepoId)`
	if !strings.Contains(buf.String(), want) {
		t.Fatalf("client RawPath arguments are not ordered by path: want %q in\n%s", want, buf.String())
	}
}

func TestClientTemplateWithOnlySSERPCsOmitsImports(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
type StreamReq {
    required string id
}

type StreamResp {
    required string message
}

sse Stream(StreamReq) StreamResp {
    method="POST"
    path="/stream"
    contentType="json"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	spec, err := Convert(dir)
	if err != nil {
		t.Fatal(err)
	}
	rpcs := clientRPCs(spec.RPCs)
	if len(rpcs) != 0 {
		t.Fatalf("expected no client RPCs for SSE-only project, got %d", len(rpcs))
	}

	var buf bytes.Buffer
	err = clientTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"RPCs":    rpcs,
	})
	if err != nil {
		t.Fatal(err)
	}
	src := buf.String()
	if strings.Contains(src, "import (") {
		t.Fatalf("SSE-only client should not emit unused imports:\n%s", src)
	}
	if !strings.Contains(src, "type Client struct") {
		t.Fatalf("SSE-only client should still emit Client type:\n%s", src)
	}
}

func TestServerTemplateWithoutRPCsOmitsUnusedImports(t *testing.T) {
	var buf bytes.Buffer
	err := serverTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Service": "Empty",
		"RPCs":    []RPC{},
	})
	if err != nil {
		t.Fatal(err)
	}
	src := buf.String()
	for _, name := range []string{`"context"`, `"net/http"`} {
		if strings.Contains(src, name) {
			t.Fatalf("empty server should not emit unused import %s:\n%s", name, src)
		}
	}
	if !strings.Contains(src, `"go-spring.org/stdlib/httpsvr"`) {
		t.Fatalf("server should still import httpsvr for Routers signature:\n%s", src)
	}
}

func TestTypeTemplateEnumAsStringUnquotesJSONString(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
enum Level {
    Junior = 1
    Senior = 2
}
`)

	spec, err := Convert(dir)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err = typeTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Consts":  spec.Consts["test.idl"],
		"Enums":   spec.Enums["test.idl"],
		"Structs": spec.Types["test.idl"],
	})
	if err != nil {
		t.Fatal(err)
	}
	src := buf.String()
	if !strings.Contains(src, "str, err := strconv.Unquote(string(data))") {
		t.Fatalf("enum string decoder should unquote JSON strings:\n%s", src)
	}
	if strings.Contains(src, "strings.Trim(string(data), \"\\\"\")") {
		t.Fatalf("enum string decoder should not trim quotes manually:\n%s", src)
	}
}

func TestConvertRejectsDuplicateMethodPath(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
type Req {
    required string id (path="id")
}

type Resp {
    required string ok
}

rpc GetByColon(Req) Resp {
    method="GET"
    path="/things/:id"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}

rpc GetByBrace(Req) Resp {
    method="GET"
    path="/things/{id}"
    contentType="form"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	_, err := Convert(dir)
	if err == nil {
		t.Fatal("expected duplicate method/path error")
	}
	if !strings.Contains(err.Error(), "duplicate RPC route GET /things/{id}") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateTemplateUsesGoParamTypes(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
enum Level {
    Junior = 1
    Senior = 2
}

type Req {
    required int age (validate="Positive($)")
    int score (validate="Score($)")
    Level level (enum_as_string, validate="ValidLevel($)")
}
`)

	spec, err := Convert(dir)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]string{
		"Positive":   "int64",
		"Score":      "int64",
		"ValidLevel": "LevelAsString",
	}
	for name, want := range tests {
		got, ok := spec.Funcs[name]
		if !ok {
			t.Fatalf("missing validate func %s", name)
		}
		if got.ParamType != want {
			t.Fatalf("validate func %s param type = %s, want %s", name, got.ParamType, want)
		}
	}

	var buf bytes.Buffer
	err = validateTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Funcs":   spec.Funcs,
	})
	if err != nil {
		t.Fatal(err)
	}
	src := buf.String()
	for name, typ := range tests {
		want := "var " + name + " = func (" + typ + ") bool"
		if !strings.Contains(src, want) {
			t.Fatalf("validate template missing %q in\n%s", want, src)
		}
	}
}

func TestConvertRejectsValidateFuncWithDifferentGoTypes(t *testing.T) {
	dir := t.TempDir()
	writeGeneratorTestProject(t, dir, `
enum Level {
    Junior = 1
    Senior = 2
}

type Req {
    Level raw (validate="Check($)")
    Level text (enum_as_string, validate="Check($)")
}
`)

	_, err := Convert(dir)
	if err == nil {
		t.Fatal("expected validate function type conflict")
	}
	if !strings.Contains(err.Error(), "validate function Check is used with different Go types") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeGeneratorTestProject(t *testing.T, dir, idl string) {
	t.Helper()
	writeGeneratorTestFile(t, dir, "meta.json", `{
  "name": "GeneratorTest",
  "description": "Generator test project",
  "version": "v1"
}`)
	writeGeneratorTestFile(t, dir, "test.idl", idl)
}

func writeGeneratorTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm); err != nil {
		t.Fatal(err)
	}
}
