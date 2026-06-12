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
	"slices"
	"strings"
	"testing"

	"github.com/go-spring/gs-http-gen/lib/httpidl"
)

func TestBuildFromExamples(t *testing.T) {
	project, err := httpidl.ParseDir("../../../examples/idl")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := build(project)
	if err != nil {
		t.Fatal(err)
	}

	if doc.OpenAPI != "3.0.3" {
		t.Fatalf("expected OpenAPI 3.0.3, got %s", doc.OpenAPI)
	}
	if doc.Info.Title != "Manager" {
		t.Fatalf("expected title Manager, got %s", doc.Info.Title)
	}

	listManagers := doc.Paths["/managers"]["get"]
	if listManagers.OperationID != "ListManagers" {
		t.Fatalf("expected ListManagers operation, got %s", listManagers.OperationID)
	}
	if listManagers.RequestBody != nil {
		t.Fatal("GET /managers should not have requestBody")
	}
	if !hasParameter(listManagers.Parameters, "page", "query", true, "integer") {
		t.Fatalf("GET /managers should have required integer query parameter page: %#v", listManagers.Parameters)
	}
	if !hasParameter(listManagers.Parameters, "department", "query", false, "integer") {
		t.Fatalf("GET /managers should have optional integer query parameter department: %#v", listManagers.Parameters)
	}

	createManager := doc.Paths["/managers"]["post"]
	if createManager.RequestBody == nil {
		t.Fatal("POST /managers should have requestBody")
	}
	createBody := createManager.RequestBody.Content["application/json"].Schema
	if !slices.Contains(createBody.Required, "name") {
		t.Fatalf("POST /managers body should require name: %#v", createBody.Required)
	}
	if got := createBody.Properties["primaryContact"].Ref; got != "#/components/schemas/ContactInfo" {
		t.Fatalf("expected primaryContact ref ContactInfo, got %s", got)
	}

	stream := doc.Paths["/assistant/stream"]["post"]
	if !stream.XSSE {
		t.Fatal("SSE operation should have x-sse=true")
	}
	if _, ok := stream.Responses["200"].Content["text/event-stream"]; !ok {
		t.Fatalf("SSE response should use text/event-stream: %#v", stream.Responses["200"].Content)
	}

	manager := doc.Components.Schemas["Manager"]
	if got := manager.Properties["level"].Type; got != "string" {
		t.Fatalf("enum_as_string field should be string enum, got %s", got)
	}
	if !slices.Contains(manager.Properties["level"].Enum, any("JUNIOR")) {
		t.Fatalf("enum_as_string field should contain enum names: %#v", manager.Properties["level"].Enum)
	}
	primaryContact := manager.Properties["primaryContact"]
	if primaryContact.Ref != "" {
		t.Fatalf("ref field with metadata should not emit direct $ref sibling fields: %#v", primaryContact)
	}
	if got := primaryContact.Description; got != "Primary contact information" {
		t.Fatalf("expected primaryContact description, got %q", got)
	}
	if len(primaryContact.AllOf) != 1 || primaryContact.AllOf[0].Ref != "#/components/schemas/ContactInfo" {
		t.Fatalf("expected primaryContact allOf ref ContactInfo, got %#v", primaryContact.AllOf)
	}

	managerLevel := doc.Components.Schemas["ManagerLevel"]
	if got := managerLevel.Type; got != "integer" {
		t.Fatalf("enum component should be integer, got %s", got)
	}
	if !slices.Contains(managerLevel.XEnumNames, "SENIOR") {
		t.Fatalf("enum component should include x-enumNames: %#v", managerLevel.XEnumNames)
	}

	payload := doc.Components.Schemas["Payload"]
	if payload.Type != "object" {
		t.Fatalf("Payload should be emitted as object wrapper, got %s", payload.Type)
	}
	if len(payload.OneOf) != 0 {
		t.Fatalf("Payload should not be emitted as bare oneOf variants: %#v", payload.OneOf)
	}
	if !slices.Contains(payload.Required, "FieldType") {
		t.Fatalf("Payload should require FieldType: %#v", payload.Required)
	}
	fieldType := payload.Properties["FieldType"]
	if fieldType.Type != "string" || !slices.Contains(fieldType.Enum, any("MessageDelta")) {
		t.Fatalf("Payload FieldType should be string enum: %#v", fieldType)
	}
	if got := payload.Properties["MessageDelta"].Ref; got != "#/components/schemas/MessageDelta" {
		t.Fatalf("Payload should include MessageDelta wrapper field ref, got %s", got)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	doc, err := build(project)
	if err != nil {
		t.Fatal(err)
	}
	servers, ok := doc.Servers.([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("expected one server, got %#v", doc.Servers)
	}
	server, ok := servers[0].(map[string]any)
	if !ok || server["url"] != "https://api.example.com" {
		t.Fatalf("unexpected server config: %#v", servers[0])
	}
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
	if err != nil {
		t.Fatal(err)
	}
	doc, err := build(project)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Paths["/ping/{id}"]["get"]; !ok {
		t.Fatalf("expected normalized OpenAPI path /ping/{id}, got %#v", doc.Paths)
	}
	if _, ok := doc.Paths["/ping/:id"]; ok {
		t.Fatalf("colon path should not be emitted in OpenAPI paths: %#v", doc.Paths)
	}
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
	if err != nil {
		t.Fatal(err)
	}
	_, err = build(project)
	if err == nil {
		t.Fatal("expected duplicate method/path error")
	}
	if !strings.Contains(err.Error(), "duplicate OpenAPI operation for GET /ping/{id}") {
		t.Fatalf("unexpected error: %v", err)
	}
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
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), os.ModePerm); err != nil {
		t.Fatal(err)
	}
}
