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
	"fmt"

	"github.com/go-spring/go-spring/spring-web"
)

//
// Swagger 流式构建器
//
type SwaggerBuilder struct {
	doc     string
	method  string
	path    string
	fn      SpringWeb.Handler
	filters []SpringWeb.Filter
}

//
// 工厂函数
//
func NewSwaggerBuilder(method string, path string, fn SpringWeb.Handler, filters []SpringWeb.Filter) *SwaggerBuilder {

	// TODO 这里需要有一个 SwaggerBuilder 的注册机制

	return &SwaggerBuilder{
		method:  method,
		path:    path,
		fn:      fn,
		filters: filters,
	}
}

//
// ***
//
func (builder *SwaggerBuilder) Doc(doc string) *SwaggerBuilder {
	builder.doc = doc
	fmt.Println(doc)
	return builder
}

//
// 返回 WebContainer.HttpMethod 需要的参数，得益于 golang 的多返回值特性
//
func (builder *SwaggerBuilder) Build() (string, SpringWeb.Handler) {
	return builder.path, func(ctx SpringWeb.WebContext) {
		SpringWeb.InvokeHandler(ctx, builder.fn, builder.filters)
	}
}

//
// 对应 WebContainer.GET 方法
//
func GET(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("GET", path, fn, filters)
}

//
// 对应 WebContainer.POST 方法
//
func POST(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("POST", path, fn, filters)
}

//
// 对应 WebContainer.PATCH 方法
//
func PATCH(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("PATCH", path, fn, filters)
}

//
// 对应 WebContainer.PUT 方法
//
func PUT(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("PUT", path, fn, filters)
}

//
// 对应 WebContainer.DELETE 方法
//
func DELETE(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("DELETE", path, fn, filters)
}

//
// 对应 WebContainer.HEAD 方法
//
func HEAD(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("HEAD", path, fn, filters)
}

//
// 对应 WebContainer.OPTIONS 方法
//
func OPTIONS(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *SwaggerBuilder {
	return NewSwaggerBuilder("OPTIONS", path, fn, filters)
}
