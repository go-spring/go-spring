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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/go-spring/gs-http-gen/lib/httpidl"
	"github.com/go-spring/gs-http-gen/lib/pathidl"
	"github.com/go-spring/stdlib/errutil"
)

// Config holds the configuration options for OpenAPI document generation.
type Config struct {
	IDLSrcDir string // Directory containing source IDL files
	OutputDir string // Directory where generated documents will be written
}

type document struct {
	OpenAPI    string     `json:"openapi"`
	Info       info       `json:"info"`
	Servers    any        `json:"servers,omitempty"`
	Paths      paths      `json:"paths"`
	Components components `json:"components"`
}

type info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
	Contact     any    `json:"contact,omitempty"`
	License     any    `json:"license,omitempty"`
}

type paths map[string]pathItem

type pathItem map[string]operation

type operation struct {
	OperationID string              `json:"operationId,omitempty"`
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	Parameters  []parameter         `json:"parameters,omitempty"`
	RequestBody *requestBody        `json:"requestBody,omitempty"`
	Responses   map[string]response `json:"responses"`
	Tags        []string            `json:"tags,omitempty"`
	ExternalDoc any                 `json:"externalDocs,omitempty"`
	XSSE        bool                `json:"x-sse,omitempty"`
}

type parameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Deprecated  bool   `json:"deprecated,omitempty"`
	Schema      schema `json:"schema"`
}

type requestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]mediaType `json:"content"`
}

type response struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content,omitempty"`
}

type mediaType struct {
	Schema schema `json:"schema"`
}

type components struct {
	Schemas map[string]schema `json:"schemas"`
}

type schema struct {
	Ref                  string            `json:"$ref,omitempty"`
	Type                 string            `json:"type,omitempty"`
	Format               string            `json:"format,omitempty"`
	Description          string            `json:"description,omitempty"`
	Deprecated           bool              `json:"deprecated,omitempty"`
	Properties           map[string]schema `json:"properties,omitempty"`
	Required             []string          `json:"required,omitempty"`
	Items                *schema           `json:"items,omitempty"`
	AdditionalProperties *schema           `json:"additionalProperties,omitempty"`
	Enum                 []any             `json:"enum,omitempty"`
	AllOf                []schema          `json:"allOf,omitempty"`
	OneOf                []schema          `json:"oneOf,omitempty"`
	XEnumNames           []string          `json:"x-enumNames,omitempty"`
}

// Gen generates openapi.json for the IDL project.
func Gen(config *Config) error {
	project, err := httpidl.ParseDir(config.IDLSrcDir)
	if err != nil {
		return errutil.Explain(err, "parse IDL project error")
	}

	doc, err := build(project)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return errutil.Explain(err, "marshal OpenAPI document error")
	}
	b = append(b, '\n')

	if err = os.MkdirAll(config.OutputDir, os.ModePerm); err != nil {
		return errutil.Explain(err, "create output dir %s error", config.OutputDir)
	}
	fileName := filepath.Join(config.OutputDir, "openapi.json")
	if err = os.WriteFile(fileName, b, os.ModePerm); err != nil {
		return errutil.Explain(err, "write file %s error", fileName)
	}
	return nil
}

func build(project httpidl.Project) (document, error) {
	doc := document{
		OpenAPI: "3.0.3",
		Info: info{
			Title:       project.Meta.Name,
			Description: project.Meta.Description,
			Version:     project.Meta.Version,
		},
		Paths: paths{},
		Components: components{
			Schemas: map[string]schema{},
		},
	}

	applyOpenAPIConfig(&doc, project.Meta.Config)

	for _, e := range sortedEnums(project.Files) {
		doc.Components.Schemas[e.Name] = enumSchema(e, false)
	}
	for _, t := range sortedTypes(project.Files) {
		if t.GenericParam != nil {
			continue
		}
		doc.Components.Schemas[t.Name] = typeSchema(project.Files, t, true)
	}

	for _, r := range sortedRPCs(project.Files) {
		op, err := buildOperation(project.Files, r)
		if err != nil {
			return document{}, errutil.Explain(err, "build operation %s error", r.Name)
		}
		path := pathidl.Format(r.PathSegments, pathidl.Brace)
		method := strings.ToLower(r.Method)
		if doc.Paths[path] == nil {
			doc.Paths[path] = pathItem{}
		}
		if prev, ok := doc.Paths[path][method]; ok {
			return document{}, fmt.Errorf("duplicate OpenAPI operation for %s %s: %s conflicts with %s", r.Method, path, r.Name, prev.OperationID)
		}
		doc.Paths[path][method] = op
	}

	return doc, nil
}

func applyOpenAPIConfig(doc *document, config map[string]any) {
	openAPIConfig, ok := config["openapi"].(map[string]any)
	if !ok {
		return
	}
	doc.Servers = openAPIConfig["servers"]
	doc.Info.Contact = openAPIConfig["contact"]
	doc.Info.License = openAPIConfig["license"]
}

func buildOperation(files map[string]httpidl.Document, r httpidl.RPC) (operation, error) {
	req, ok := httpidl.FindType(files, r.Request)
	if !ok {
		return operation{}, errutil.Explain(nil, "request type %s is used but not defined", r.Request)
	}

	op := operation{
		OperationID: r.Name,
		Summary:     annotationValue(r.Annotations, "summary"),
		Description: commentsText(r.Comments),
		Responses: map[string]response{
			"200": {
				Description: "OK",
				Content: map[string]mediaType{
					responseContentType(r): {
						Schema: typeDefSchema(files, r.Response, false),
					},
				},
			},
		},
		XSSE: r.SSE,
	}

	var bodyFields []httpidl.TypeField
	for _, f := range req.Type.Fields {
		if f.Binding == nil {
			bodyFields = append(bodyFields, f)
			continue
		}
		op.Parameters = append(op.Parameters, parameter{
			Name:        f.Binding.Field,
			In:          f.Binding.Source,
			Description: commentsText(f.Comments),
			Required:    f.Binding.Source == "path" || f.Required,
			Deprecated:  f.Deprecated,
			Schema:      typeDefSchema(files, f.Type, f.EnumAsString),
		})
	}

	if len(bodyFields) > 0 {
		contentType := r.ContentType
		op.RequestBody = &requestBody{
			Required: hasRequiredField(bodyFields),
			Content: map[string]mediaType{
				contentType: {
					Schema: objectSchemaFromFields(files, bodyFields, strings.HasPrefix(contentType, "application/x-www-form-urlencoded")),
				},
			},
		}
	}

	return op, nil
}

func responseContentType(r httpidl.RPC) string {
	if r.SSE {
		return "text/event-stream"
	}
	return "application/json"
}

func typeSchema(files map[string]httpidl.Document, t httpidl.Type, useJSONName bool) schema {
	if t.OneOf {
		s := objectSchemaFromFields(files, t.Fields, !useJSONName)
		s.Description = commentsText(t.Comments)
		return s
	}
	s := objectSchemaFromFields(files, t.Fields, !useJSONName)
	s.Description = commentsText(t.Comments)
	return s
}

func objectSchemaFromFields(files map[string]httpidl.Document, fields []httpidl.TypeField, useFormName bool) schema {
	s := schema{
		Type:       "object",
		Properties: map[string]schema{},
	}
	for _, f := range fields {
		name := f.JSONTag.Name
		if useFormName {
			name = f.FormTag.Name
		}
		s.Properties[name] = typeFieldSchema(files, f)
		if f.Required {
			s.Required = append(s.Required, name)
		}
	}
	sort.Strings(s.Required)
	return s
}

func typeFieldSchema(files map[string]httpidl.Document, f httpidl.TypeField) schema {
	s := typeDefSchema(files, f.Type, f.EnumAsString)
	description := commentsText(f.Comments)
	if s.Ref != "" && (description != "" || f.Deprecated) {
		return schema{
			Description: description,
			Deprecated:  f.Deprecated,
			AllOf: []schema{
				{Ref: s.Ref},
			},
		}
	}
	s.Description = description
	s.Deprecated = f.Deprecated
	return s
}

func typeDefSchema(files map[string]httpidl.Document, t httpidl.TypeDefinition, enumAsString bool) schema {
	switch typ := t.(type) {
	case httpidl.BaseType:
		return baseSchema(typ.Name)
	case httpidl.BytesType:
		return schema{Type: "string", Format: "byte"}
	case httpidl.UserType:
		if e, ok := httpidl.FindEnum(files, typ.Name); ok {
			return enumSchema(e.Type, enumAsString)
		}
		return schema{Ref: "#/components/schemas/" + typ.Name}
	case httpidl.ListType:
		item := typeDefSchema(files, typ.Item, false)
		return schema{Type: "array", Items: &item}
	case httpidl.MapType:
		value := typeDefSchema(files, typ.Value, false)
		return schema{Type: "object", AdditionalProperties: &value}
	case httpidl.InstType:
		if src, ok := httpidl.FindType(files, typ.BaseName); ok && src.Type.GenericParam != nil {
			fields := make([]httpidl.TypeField, 0, len(src.Type.Fields))
			for _, f := range src.Type.Fields {
				f.Type = replaceGenericType(f.Type, *src.Type.GenericParam, typ.GenericType)
				fields = append(fields, f)
			}
			return objectSchemaFromFields(files, fields, false)
		}
		return schema{}
	default:
		return schema{}
	}
}

func replaceGenericType(t httpidl.TypeDefinition, genericName string, genericType httpidl.TypeDefinition) httpidl.TypeDefinition {
	switch u := t.(type) {
	case httpidl.UserType:
		if u.Name == genericName {
			return genericType
		}
		return u
	case httpidl.ListType:
		u.Item = replaceGenericType(u.Item, genericName, genericType)
		return u
	case httpidl.MapType:
		u.Value = replaceGenericType(u.Value, genericName, genericType)
		return u
	default:
		return t
	}
}

func baseSchema(name string) schema {
	switch name {
	case "bool":
		return schema{Type: "boolean"}
	case "int", "uint":
		return schema{Type: "integer", Format: "int64"}
	case "float":
		return schema{Type: "number", Format: "double"}
	case "string":
		return schema{Type: "string"}
	default:
		return schema{}
	}
}

func enumSchema(e httpidl.Enum, asString bool) schema {
	s := schema{
		Description: commentsText(e.Comments),
	}
	for _, f := range e.Fields {
		if asString {
			s.Type = "string"
			s.Enum = append(s.Enum, f.Name)
		} else {
			s.Type = "integer"
			s.Format = "int64"
			s.Enum = append(s.Enum, f.Value)
			s.XEnumNames = append(s.XEnumNames, f.Name)
		}
	}
	return s
}

func hasRequiredField(fields []httpidl.TypeField) bool {
	for _, f := range fields {
		if f.Required {
			return true
		}
	}
	return false
}

func annotationValue(annotations []httpidl.Annotation, name string) string {
	a, ok := httpidl.FindAnnotation(annotations, name)
	if !ok || a.Value == nil {
		return ""
	}
	s, err := strconv.Unquote(strings.TrimSpace(*a.Value))
	if err != nil {
		return strings.Trim(strings.TrimSpace(*a.Value), `"`)
	}
	return s
}

func commentsText(c httpidl.Comments) string {
	var lines []string
	for _, block := range c.Above {
		lines = append(lines, cleanCommentLines(block.Text)...)
	}
	if c.Right != nil {
		lines = append(lines, cleanCommentLines(c.Right.Text)...)
	}
	return strings.Join(lines, "\n")
}

func cleanCommentLines(in []string) []string {
	var ret []string
	for _, line := range in {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimPrefix(line, "#")
		line = strings.TrimPrefix(line, "/*")
		line = strings.TrimSuffix(line, "*/")
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "*"))
		if line == "" {
			continue
		}
		ret = append(ret, line)
	}
	return ret
}

func sortedEnums(files map[string]httpidl.Document) []httpidl.Enum {
	var ret []httpidl.Enum
	for _, doc := range files {
		for _, e := range doc.Enums {
			if e.Kind == httpidl.EnumKindExtends {
				continue
			}
			ret = append(ret, e)
		}
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret
}

func sortedTypes(files map[string]httpidl.Document) []httpidl.Type {
	var ret []httpidl.Type
	for _, doc := range files {
		ret = append(ret, doc.Types...)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Name < ret[j].Name
	})
	return ret
}

func sortedRPCs(files map[string]httpidl.Document) []httpidl.RPC {
	var ret []httpidl.RPC
	for _, doc := range files {
		ret = append(ret, doc.RPCs...)
	}
	sort.Slice(ret, func(i, j int) bool {
		if ret[i].Path != ret[j].Path {
			return ret[i].Path < ret[j].Path
		}
		if ret[i].Method != ret[j].Method {
			return ret[i].Method < ret[j].Method
		}
		return ret[i].Name < ret[j].Name
	})
	return ret
}
