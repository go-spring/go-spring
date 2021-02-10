/*
 * Copyright 2012-2019 the original author or authors.
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

package SpringSwagger

import (
	"reflect"

	"github.com/go-openapi/spec"
)

type bindParam struct {
	param       interface{}
	description string
}

// Operation 封装 *spec.Operation 对象，提供更多功能
type Operation struct {
	*spec.Operation
	bindParam *bindParam
}

// NewOperation creates a new operation instance.
func NewOperation(id string) *Operation {
	return &Operation{Operation: spec.NewOperation(id)}
}

// WithID sets the ID property on this operation, allows for chaining.
func (o *Operation) WithID(id string) *Operation {
	o.Operation.WithID(id)
	return o
}

// WithDescription sets the description on this operation, allows for chaining
func (o *Operation) WithDescription(description string) *Operation {
	o.Operation.WithDescription(description)
	return o
}

// WithSummary sets the summary on this operation, allows for chaining
func (o *Operation) WithSummary(summary string) *Operation {
	o.Operation.WithSummary(summary)
	return o
}

// WithExternalDocs sets/removes the external docs for/from this operation.
func (o *Operation) WithExternalDocs(description, url string) *Operation {
	o.Operation.WithExternalDocs(description, url)
	return o
}

// Deprecate marks the operation as deprecated
func (o *Operation) Deprecate() *Operation {
	o.Operation.Deprecate()
	return o
}

// Undeprecate marks the operation as not deprecated
func (o *Operation) Undeprecate() *Operation {
	o.Operation.Undeprecate()
	return o
}

// WithConsumes adds media types for incoming body values
func (o *Operation) WithConsumes(mediaTypes ...string) *Operation {
	o.Operation.WithConsumes(mediaTypes...)
	return o
}

// WithProduces adds media types for outgoing body values
func (o *Operation) WithProduces(mediaTypes ...string) *Operation {
	o.Operation.WithProduces(mediaTypes...)
	return o
}

// WithTags adds tags for this operation
func (o *Operation) WithTags(tags ...string) *Operation {
	o.Operation.WithTags(tags...)
	return o
}

// SetSchemes 设置服务协议
func (o *Operation) WithSchemes(schemes ...string) *Operation {
	o.Operation.Schemes = schemes
	return o
}

// AddParam adds a parameter to this operation
func (o *Operation) AddParam(param *spec.Parameter) *Operation {
	o.Operation.AddParam(param)
	return o
}

// RemoveParam removes a parameter from the operation
func (o *Operation) RemoveParam(name, in string) *Operation {
	o.Operation.RemoveParam(name, in)
	return o
}

// SecuredWith adds a security scope to this operation.
func (o *Operation) SecuredWith(name string, scopes ...string) *Operation {
	o.Operation.SecuredWith(name, scopes...)
	return o
}

// WithDefaultResponse adds a default response to the operation.
func (o *Operation) WithDefaultResponse(response *spec.Response) *Operation {
	o.Operation.WithDefaultResponse(response)
	return o
}

// RespondsWith adds a status code response to the operation.
func (o *Operation) RespondsWith(code int, response *spec.Response) *Operation {
	o.Operation.RespondsWith(code, response)
	return o
}

// Bind 绑定请求参数
func (o *Operation) BindParam(i interface{}, description string) *Operation {
	o.bindParam = &bindParam{param: i, description: description}
	return o
}

// parseBind 解析绑定的请求参数
func (o *Operation) parseBind() error {
	if o.bindParam != nil && o.bindParam.param != nil {
		t := reflect.TypeOf(o.bindParam.param)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() == reflect.Struct {
			schema := spec.RefSchema("#/definitions/" + t.Name())
			param := BodyParam("body", schema).
				WithDescription(o.bindParam.description).
				AsRequired()
			o.AddParam(param)
		}
	}
	return nil
}

// HeaderParam creates a header parameter, this is always required by default
func HeaderParam(name string, typ, format string) *spec.Parameter {
	param := spec.HeaderParam(name)
	param.Typed(typ, format)
	return param
}

// PathParam creates a path parameter, this is always required
func PathParam(name string, typ, format string) *spec.Parameter {
	param := spec.PathParam(name)
	param.Typed(typ, format)
	return param
}

// BodyParam creates a body parameter
func BodyParam(name string, schema *spec.Schema) *spec.Parameter {
	return &spec.Parameter{ParamProps: spec.ParamProps{Name: name, In: "body", Schema: schema}}
}

// NewResponse creates a new response instance
func NewResponse(description string) *spec.Response {
	resp := new(spec.Response)
	resp.Description = description
	return resp
}

// NewBindResponse creates a new response instance
func NewBindResponse(i interface{}, description string) *spec.Response {
	resp := new(spec.Response)
	resp.Description = description

	t := reflect.TypeOf(i)

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	slice := false
	if t.Kind() == reflect.Slice {
		slice = true
		t = t.Elem()
	}

	if t.Kind() == reflect.Struct {
		schema := spec.RefSchema("#/definitions/" + t.Name())
		if slice {
			resp.WithSchema(spec.ArrayProperty(schema))
		} else {
			resp.WithSchema(schema)
		}
	}

	return resp
}
