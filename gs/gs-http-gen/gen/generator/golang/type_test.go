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
	"go-spring.org/stdlib/testing/require"
)

func TestGenDecodeJSONMapKey(t *testing.T) {
	got := genDecodeJSON("map[int64]string", []TypeKind{
		TypeKindMap,
		TypeKindInt,
		TypeKindString,
	})
	require.String(t, got).Equal("jsonflow.DecodeMap(jsonflow.DecodeIntKey[int64], jsonflow.DecodeString)")
}

func TestGenDecodeJSONAny(t *testing.T) {
	got := genDecodeJSON("any", []TypeKind{TypeKindAny})
	require.String(t, got).Equal("jsonflow.DecodeAny[any]")
}

func TestGenEncodeJSONMap(t *testing.T) {
	got := genEncodeJSON("map[string][]int64", []TypeKind{
		TypeKindMap,
		TypeKindString,
		TypeKindList,
		TypeKindInt,
	})
	require.String(t, got).Equal("jsonflow.EncodeMap(jsonflow.EncodeStringKey[string], jsonflow.EncodeArray(jsonflow.EncodeInt[int64]))")
}

func TestGenDecodeJSONRequiredCheckBytes(t *testing.T) {
	got := genDecodeJSONRequiredCheck("r.Avatar", []TypeKind{TypeKindBytes}, "avatar")
	want := `if r.Avatar == nil {
			return errutil.Explain(nil, "field \"avatar\" must not be null")
		}`
	require.String(t, got).Equal(want)
}

func TestGenValidateExprWithCustomFunc(t *testing.T) {
	// Test that $ is correctly replaced with field value (x.Age)
	expr, err := validate.Parse("Positive($) && $ > 0")
	require.Error(t, err).Nil()

	got, err := genValidateExpr("User", "Age", "int64", expr)
	require.Error(t, err).Nil()

	// Check that $ was replaced with x.Age (not just "Age")
	require.String(t, got).Contains("Positive(x.Age)")
	require.String(t, got).Contains("x.Age > 0")

	// Check that error propagation is correct
	require.String(t, got).Contains("ok, err := Positive(x.Age)")
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
			require.Error(t, err).Nil()
			got, err := compileBoolExpr("x.Name", expr)
			require.Error(t, err).Nil()
			require.String(t, got).Equal(tt.want)
		})
	}
}

func TestGenValidateNestedMapValue(t *testing.T) {
	got := genValidateNested("Req", "Contacts", "x.Contacts", []TypeKind{
		TypeKindMap,
		TypeKindString,
		TypeKindStructPtr,
	}, 0)
	require.String(t, got).Contains("for _, v0 := range x.Contacts")
	require.String(t, got).Contains("v0.Validate()")
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
	require.Error(t, err).Nil()

	var buf bytes.Buffer
	err = typeTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Consts":  spec.Consts["test.idl"],
		"Enums":   spec.Enums["test.idl"],
		"Structs": spec.Types["test.idl"],
	})
	require.Error(t, err).Nil()
	src := buf.String()
	require.String(t, src).Contains("func (x *Child) Validate() error")
	require.String(t, src).Contains("x.Child.Validate()")
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
	require.Error(t, err).Nil()
	require.That(t, len(spec.RPCs)).Equal(1)
	gotFields := strings.Join(spec.RPCs[0].PathParamFields, ",")
	require.String(t, gotFields).Equal("OrgId,RepoId")

	var buf bytes.Buffer
	err = clientTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"RPCs":    spec.RPCs,
	})
	require.Error(t, err).Nil()
	require.String(t, buf.String()).Contains(`RawPath: fmt.Sprintf("/org/%v/repos/%v", req.OrgId, req.RepoId)`)
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
	require.Error(t, err).Nil()
	rpcs := clientRPCs(spec.RPCs)
	require.That(t, len(rpcs)).Equal(0)

	var buf bytes.Buffer
	err = clientTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"RPCs":    rpcs,
	})
	require.Error(t, err).Nil()
	src := buf.String()
	require.That(t, strings.Contains(src, "import (")).False()
	require.String(t, src).Contains("type Client struct")
}

func TestServerTemplateWithoutRPCsOmitsUnusedImports(t *testing.T) {
	var buf bytes.Buffer
	err := serverTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Service": "Empty",
		"RPCs":    []RPC{},
	})
	require.Error(t, err).Nil()
	src := buf.String()
	for _, name := range []string{`"context"`, `"net/http"`} {
		require.That(t, strings.Contains(src, name)).False()
	}
	require.String(t, src).Contains(`"go-spring.org/spring/web/httpsvr"`)
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
	require.Error(t, err).Nil()

	var buf bytes.Buffer
	err = typeTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Consts":  spec.Consts["test.idl"],
		"Enums":   spec.Enums["test.idl"],
		"Structs": spec.Types["test.idl"],
	})
	require.Error(t, err).Nil()
	src := buf.String()
	require.String(t, src).Contains("str, err := strconv.Unquote(string(data))")
	require.That(t, strings.Contains(src, "strings.Trim(string(data), \"\\\"\")")).False()
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
	require.Error(t, err).NotNil()
	require.String(t, err.Error()).Contains("duplicate RPC route GET /things/{id}")
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
	require.Error(t, err).Nil()
	tests := map[string]string{
		"Positive":   "int64",
		"Score":      "int64",
		"ValidLevel": "LevelAsString",
	}
	for name, want := range tests {
		got, ok := spec.Funcs[name]
		require.That(t, ok).True()
		require.String(t, got.ParamType).Equal(want)
	}

	var buf bytes.Buffer
	err = validateTmpl.Execute(&buf, map[string]any{
		"Package": "proto",
		"Funcs":   spec.Funcs,
	})
	require.Error(t, err).Nil()
	src := buf.String()
	for name, typ := range tests {
		want := "var " + name + " = func (" + typ + ") (bool, error)"
		require.String(t, src).Contains(want)
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
	require.Error(t, err).NotNil()
	require.String(t, err.Error()).Contains("validate function Check is used with different Go types")
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
	require.Error(t, os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm)).Nil()
}
