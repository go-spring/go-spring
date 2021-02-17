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

package app

import (
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/web"
)

// webMapping Web 路由映射表
type webMapping map[string]*WebMapper

func (m webMapping) addMapper(mapping *WebMapper) {
	m[mapping.Key()] = mapping
}

// HandleRequest 路由注册
func (m webMapping) HandleRequest(method uint32, path string, fn web.Handler, filters []bean.Selector) *WebMapper {
	mapping := NewWebMapper(method, path, fn, filters)
	m.addMapper(mapping)
	return mapping
}

// WebMapper 封装 Web 路由映射
type WebMapper struct {
	mapper  *web.Mapper
	filters []bean.Selector
}

// NewWebMapper WebMapper 的构造函数
func NewWebMapper(method uint32, path string, handler web.Handler, filters []bean.Selector) *WebMapper {
	return &WebMapper{mapper: web.NewMapper(method, path, handler, nil), filters: filters}
}

// Mapper 返回封装的 Mapper 对象
func (m *WebMapper) Mapper() *web.Mapper {
	return m.mapper
}

// Key 返回 WebMapper 的标识符
func (m *WebMapper) Key() string {
	return m.mapper.Key()
}

// Method 返回 WebMapper 的方法
func (m *WebMapper) Method() uint32 {
	return m.mapper.Method()
}

// Path 返回 WebMapper 的路径
func (m *WebMapper) Path() string {
	return m.mapper.Path()
}

// HandlerSelector 返回处理函数选择器
func (m *WebMapper) Handler() web.Handler {
	return m.mapper.Handler()
}

// Filters 返回 WebMapper 的过滤器列表
func (m *WebMapper) Filters() []bean.Selector {
	return m.filters
}

//// Swagger 生成并返回 Swagger 操作节点
//func (m *WebMapper) Swagger() *web.Operation {
//	return m.mapper.Swagger("")
//}

// WebRouter 路由分组
type WebRouter struct {
	mapping  webMapping
	basePath string
	filters  []bean.Selector
}

// NewWebRouter WebRouter 的构造函数
func NewWebRouter(mapping webMapping, basePath string, filters []bean.Selector) *WebRouter {
	return &WebRouter{mapping: mapping, basePath: basePath, filters: filters}
}

// Route 创建子路由分组
func (r *WebRouter) Route(basePath string, filters ...bean.Selector) *WebRouter {
	return &WebRouter{
		mapping:  r.mapping,
		basePath: r.basePath + basePath,
		filters:  append(r.filters, filters...),
	}
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *WebRouter) HandleRequest(method uint32, path string, fn web.Handler, filters ...bean.Selector) *WebMapper {
	filters = append(r.filters, filters...) // 组合 WebRouter 和 WebMapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func (r *WebRouter) MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(method, path, web.FUNC(fn), filters...)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func (r *WebRouter) BindingRequest(method uint32, path string, fn interface{}, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(method, path, web.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *WebRouter) HandleGet(path string, fn web.Handler, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func (r *WebRouter) MappingGet(path string, fn web.HandlerFunc, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func (r *WebRouter) BindingGet(path string, fn interface{}, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *WebRouter) HandlePost(path string, fn web.Handler, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func (r *WebRouter) MappingPost(path string, fn web.HandlerFunc, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func (r *WebRouter) BindingPost(path string, fn interface{}, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *WebRouter) HandlePut(path string, fn web.Handler, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func (r *WebRouter) MappingPut(path string, fn web.HandlerFunc, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func (r *WebRouter) BindingPut(path string, fn interface{}, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *WebRouter) HandleDelete(path string, fn web.Handler, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func (r *WebRouter) MappingDelete(path string, fn web.HandlerFunc, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func (r *WebRouter) BindingDelete(path string, fn interface{}, filters ...bean.Selector) *WebMapper {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}

///////////////////////////////////////////////////////////////////////////////

func (app *Application) WebMapper(mapper *WebMapper) *Application {
	app.WebMapping.addMapper(mapper)
	return app
}
