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
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/web"
)

// WebMapping 默认的 Web 路由映射表
var WebMapping = webMapping(make(map[string]*webMapper))

// Route 返回和 webMapper 绑定的路由分组
func Route(basePath string, filters ...core.BeanSelector) *webRouter {
	return newRouter(WebMapping, basePath, filters)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return WebMapping.HandleRequest(method, path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return WebMapping.HandleRequest(method, path, web.FUNC(fn), filters)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func BindingRequest(method uint32, path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return WebMapping.HandleRequest(method, path, web.BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func MappingGet(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func BindingGet(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func MappingPost(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func BindingPost(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func MappingPut(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func BindingPut(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func MappingDelete(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func BindingDelete(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}

//////////////////////////////////////////////////////////////////////////////////////////////

// webMapping Web 路由映射表
type webMapping map[string]*webMapper

// HandleRequest 路由注册
func (m webMapping) HandleRequest(method uint32, path string, fn web.Handler, filters []core.BeanSelector) *webMapper {
	mapping := newMapping(method, path, fn, filters)
	m[mapping.Key()] = mapping
	return mapping
}

// webMapper 封装 Web 路由映射
type webMapper struct {
	mapper  *web.Mapper
	filters []core.BeanSelector
}

// newMapping webMapper 的构造函数
func newMapping(method uint32, path string, handler web.Handler, filters []core.BeanSelector) *webMapper {
	return &webMapper{mapper: web.NewMapper(method, path, handler, nil), filters: filters}
}

// Mapper 返回封装的 Mapper 对象
func (m *webMapper) Mapper() *web.Mapper {
	return m.mapper
}

// Key 返回 webMapper 的标识符
func (m *webMapper) Key() string {
	return m.mapper.Key()
}

// Method 返回 webMapper 的方法
func (m *webMapper) Method() uint32 {
	return m.mapper.Method()
}

// Path 返回 webMapper 的路径
func (m *webMapper) Path() string {
	return m.mapper.Path()
}

// HandlerSelector 返回处理函数选择器
func (m *webMapper) Handler() web.Handler {
	return m.mapper.Handler()
}

// Filters 返回 webMapper 的过滤器列表
func (m *webMapper) Filters() []core.BeanSelector {
	return m.filters
}

//// Swagger 生成并返回 Swagger 操作节点
//func (m *webMapper) Swagger() *web.Operation {
//	return m.mapper.Swagger("")
//}

// webRouter 路由分组
type webRouter struct {
	mapping  webMapping
	basePath string
	filters  []core.BeanSelector
}

// newRouter webRouter 的构造函数
func newRouter(mapping webMapping, basePath string, filters []core.BeanSelector) *webRouter {
	return &webRouter{mapping: mapping, basePath: basePath, filters: filters}
}

// Route 创建子路由分组
func (r *webRouter) Route(basePath string, filters ...core.BeanSelector) *webRouter {
	return &webRouter{
		mapping:  r.mapping,
		basePath: r.basePath + basePath,
		filters:  append(r.filters, filters...),
	}
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *webRouter) HandleRequest(method uint32, path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	filters = append(r.filters, filters...) // 组合 webRouter 和 webMapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func (r *webRouter) MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(method, path, web.FUNC(fn), filters...)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func (r *webRouter) BindingRequest(method uint32, path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(method, path, web.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *webRouter) HandleGet(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func (r *webRouter) MappingGet(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func (r *webRouter) BindingGet(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *webRouter) HandlePost(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func (r *webRouter) MappingPost(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func (r *webRouter) BindingPost(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *webRouter) HandlePut(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func (r *webRouter) MappingPut(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func (r *webRouter) BindingPut(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *webRouter) HandleDelete(path string, fn web.Handler, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func (r *webRouter) MappingDelete(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func (r *webRouter) BindingDelete(path string, fn interface{}, filters ...core.BeanSelector) *webMapper {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}
