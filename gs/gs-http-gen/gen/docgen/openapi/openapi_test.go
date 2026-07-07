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

package openapi

import (
	"os"
	"path/filepath"
	"testing"

	"go-spring.org/gs-http-gen/lib/httpidl"
	"go-spring.org/stdlib/testing/require"
)

func TestBuildFromExamples(t *testing.T) {
	project, err := httpidl.ParseDir("../../../examples/idl")
	require.Error(t, err).Nil()
	doc, err := build(project)
	require.Error(t, err).Nil()

	require.String(t, doc.OpenAPI).Equal("3.0.3")
	require.String(t, doc.Info.Title).Equal("Manager")

	listManagers := doc.Paths["/managers"]["get"]
	require.String(t, listManagers.OperationID).Equal("ListManagers")
	require.That(t, listManagers.RequestBody).Nil()
	require.That(t, hasParameter(listManagers.Parameters, "page", "query", true, "integer")).True()
	require.That(t, hasParameter(listManagers.Parameters, "department", "query", false, "integer")).True()

	createManager := doc.Paths["/managers"]["post"]
	require.That(t, createManager.RequestBody).NotNil()
	createBody := createManager.RequestBody.Content["application/json"].Schema
	require.Slice(t, createBody.Required).Contains("name")
	require.String(t, createBody.Properties["primaryContact"].Ref).Equal("#/components/schemas/ContactInfo")

	stream := doc.Paths["/assistant/stream"]["post"]
	require.That(t, stream.XSSE).True()
	_, ok := stream.Responses["200"].Content["text/event-stream"]
	require.That(t, ok).True()

	manager := doc.Components.Schemas["Manager"]
	require.String(t, manager.Properties["level"].Type).Equal("string")
	require.Slice(t, manager.Properties["level"].Enum).Contains(any("JUNIOR"))
	primaryContact := manager.Properties["primaryContact"]
	require.String(t, primaryContact.Ref).Equal("")
	require.String(t, primaryContact.Description).Equal("Primary contact information")
	require.That(t, len(primaryContact.AllOf)).Equal(1)
	require.String(t, primaryContact.AllOf[0].Ref).Equal("#/components/schemas/ContactInfo")

	managerLevel := doc.Components.Schemas["ManagerLevel"]
	require.String(t, managerLevel.Type).Equal("integer")
	require.Slice(t, managerLevel.XEnumNames).Contains("SENIOR")

	payload := doc.Components.Schemas["Payload"]
	require.String(t, payload.Type).Equal("object")
	require.That(t, len(payload.OneOf)).Equal(0)
	require.Slice(t, payload.Required).Contains("FieldType")
	fieldType := payload.Properties["FieldType"]
	require.String(t, fieldType.Type).Equal("string")
	require.Slice(t, fieldType.Enum).Contains(any("MessageDelta"))
	require.String(t, payload.Properties["MessageDelta"].Ref).Equal("#/components/schemas/MessageDelta")
}

func TestBuildWithOpenAPIConfigServers(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "meta.json", `{
  "name": "Example",
  "description": "Example API",
  "version": "v1",
  "config": {
    "openapi": {
      "servers": [
        {"url": "https://api.example.com", "description": "prod"}
      ]
    }
  }
}`)
	writeTestFile(t, dir, "example.idl", `
type PingReq {
    required string id (path="id")
}

type PingResp {
    required string message
}

rpc Ping(PingReq) PingResp {
    method="GET"
    path="/ping/{id}"
    contentType="form"
    summary="Ping"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	project, err := httpidl.ParseDir(dir)
	require.Error(t, err).Nil()
	doc, err := build(project)
	require.Error(t, err).Nil()
	servers, ok := doc.Servers.([]any)
	require.That(t, ok).True()
	require.That(t, len(servers)).Equal(1)
	server, ok := servers[0].(map[string]any)
	require.That(t, ok).True()
	require.That(t, server["url"]).Equal("https://api.example.com")
}

func TestBuildNormalizesColonPathParams(t *testing.T) {
	dir := t.TempDir()
	writeTestProject(t, dir, `
type PingReq {
    required string id (path="id")
}

type PingResp {
    required string message
}

rpc Ping(PingReq) PingResp {
    method="GET"
    path="/ping/:id"
    contentType="form"
    summary="Ping"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	project, err := httpidl.ParseDir(dir)
	require.Error(t, err).Nil()
	doc, err := build(project)
	require.Error(t, err).Nil()
	_, ok := doc.Paths["/ping/{id}"]["get"]
	require.That(t, ok).True()
	_, ok = doc.Paths["/ping/:id"]
	require.That(t, ok).False()
}

func TestBuildRejectsDuplicateMethodPath(t *testing.T) {
	dir := t.TempDir()
	writeTestProject(t, dir, `
type PingReq {
    required string id (path="id")
}

type PingResp {
    required string message
}

rpc Ping(PingReq) PingResp {
    method="GET"
    path="/ping/:id"
    contentType="form"
    summary="Ping"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}

rpc GetPing(PingReq) PingResp {
    method="GET"
    path="/ping/{id}"
    contentType="form"
    summary="Get ping"

    connTimeout=100
    readTimeout=100
    writeTimeout=100
}
`)

	project, err := httpidl.ParseDir(dir)
	require.Error(t, err).Nil()
	_, err = build(project)
	require.Error(t, err).NotNil()
	require.String(t, err.Error()).Contains("duplicate OpenAPI operation for GET /ping/{id}")
}

func hasParameter(parameters []parameter, name, in string, required bool, typ string) bool {
	for _, p := range parameters {
		if p.Name == name && p.In == in && p.Required == required && p.Schema.Type == typ {
			return true
		}
	}
	return false
}

func writeTestProject(t *testing.T, dir, idl string) {
	t.Helper()
	writeTestFile(t, dir, "meta.json", `{
  "name": "Example",
  "description": "Example API",
  "version": "v1"
}`)
	writeTestFile(t, dir, "example.idl", idl)
}

func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.Error(t, os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm)).Nil()
}
