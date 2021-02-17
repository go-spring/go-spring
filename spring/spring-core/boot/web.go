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

package boot

import (
	"github.com/go-spring/spring-core/app"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/web"
)

// Route 返回和 app.WebMapper 绑定的路由分组
func Route(basePath string, filters ...bean.Selector) *app.WebRouter {
	return app.NewWebRouter(gApp.WebMapping, basePath, filters)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, web.FUNC(fn), filters)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func BindingRequest(method uint32, path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return gApp.WebMapping.HandleRequest(method, path, web.BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func MappingGet(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func BindingGet(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func MappingPost(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func BindingPost(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func MappingPut(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func BindingPut(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func MappingDelete(path string, fn web.HandlerFunc, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func BindingDelete(path string, fn interface{}, filters ...bean.Selector) *app.WebMapper {
	return HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}
