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
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/web"
)

// WebMapping Web 路由映射表
type WebMapping struct {
	Mappings map[string]*Mapping
}

// NewWebMapping WebMapping 的构造函数
func NewWebMapping() *WebMapping {
	return &WebMapping{Mappings: make(map[string]*Mapping)}
}

// HandleRequest 路由注册
func (m *WebMapping) HandleRequest(method uint32, path string, fn web.Handler, filters []core.BeanSelector) *Mapping {
	mapping := newMapping(method, path, fn, filters)
	m.Mappings[mapping.Key()] = mapping
	return mapping
}

// Mapping 封装 Web 路由映射
type Mapping struct {
	handler web.Handler
	mapper  *web.Mapper // 路由映射器
	filters []core.BeanSelector
}

// newMapping Mapping 的构造函数
func newMapping(method uint32, path string, handler web.Handler, filters []core.BeanSelector) *Mapping {
	return &Mapping{mapper: web.NewMapper(method, path, nil, nil), handler: handler, filters: filters}
}

// Mapper 返回封装的 Mapper 对象
func (m *Mapping) Mapper() *web.Mapper {
	return m.mapper
}

// Key 返回 Mapper 的标识符
func (m *Mapping) Key() string {
	return m.mapper.Key()
}

// Method 返回 Mapper 的方法
func (m *Mapping) Method() uint32 {
	return m.mapper.Method()
}

// Path 返回 Mapper 的路径
func (m *Mapping) Path() string {
	return m.mapper.Path()
}

// HandlerSelector 返回处理函数选择器
func (m *Mapping) Handler() web.Handler {
	return m.handler
}

// Filters 返回 Mapper 的过滤器列表
func (m *Mapping) Filters() []core.BeanSelector {
	return m.filters
}

//// Swagger 生成并返回 Swagger 操作节点
//func (m *Mapping) Swagger() *web.Operation {
//	return m.mapper.Swagger("")
//}

// Router 路由分组
type Router struct {
	mapping  *WebMapping
	basePath string
	filters  []core.BeanSelector
}

// newRouter Router 的构造函数
func newRouter(mapping *WebMapping, basePath string, filters []core.BeanSelector) *Router {
	return &Router{mapping: mapping, basePath: basePath, filters: filters}
}

// Route 创建子路由分组
func (r *Router) Route(basePath string, filters ...core.BeanSelector) *Router {
	return &Router{
		mapping:  r.mapping,
		basePath: r.basePath + basePath,
		filters:  append(r.filters, filters...),
	}
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *Router) HandleRequest(method uint32, path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	filters = append(r.filters, filters...) // 组合 Router 和 Mapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (r *Router) RequestMapping(method uint32, path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(method, path, web.FUNC(fn), filters...)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (r *Router) RequestBinding(method uint32, path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(method, path, web.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *Router) HandleGet(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodGet, path, fn, filters...)
}

// GetMapping 注册 GET 方法处理函数
func (r *Router) GetMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// GetBinding 注册 GET 方法处理函数
func (r *Router) GetBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *Router) HandlePost(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPost, path, fn, filters...)
}

// PostMapping 注册 POST 方法处理函数
func (r *Router) PostMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// PostBinding 注册 POST 方法处理函数
func (r *Router) PostBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *Router) HandlePut(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPut, path, fn, filters...)
}

// PutMapping 注册 PUT 方法处理函数
func (r *Router) PutMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// PutBinding 注册 PUT 方法处理函数
func (r *Router) PutBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *Router) HandleDelete(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodDelete, path, fn, filters...)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (r *Router) DeleteMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (r *Router) DeleteBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}

///////////////////// 全局函数 /////////////////////////////

// Route 返回和 Mapping 绑定的路由分组
func Route(basePath string, filters ...core.BeanSelector) *Router {
	return newRouter(gApp.webMapping, basePath, filters)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return gApp.webMapping.HandleRequest(method, path, fn, filters)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func RequestMapping(method uint32, path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return gApp.webMapping.HandleRequest(method, path, web.FUNC(fn), filters)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func RequestBinding(method uint32, path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return gApp.webMapping.HandleRequest(method, path, web.BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodGet, path, fn, filters...)
}

// GetMapping 注册 GET 方法处理函数
func GetMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// GetBinding 注册 GET 方法处理函数
func GetBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPost, path, fn, filters...)
}

// PostMapping 注册 POST 方法处理函数
func PostMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// PostBinding 注册 POST 方法处理函数
func PostBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPut, path, fn, filters...)
}

// PutMapping 注册 PUT 方法处理函数
func PutMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// PutBinding 注册 PUT 方法处理函数
func PutBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodDelete, path, fn, filters...)
}

// DeleteMapping 注册 DELETE 方法处理函数
func DeleteMapping(path string, fn web.HandlerFunc, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// DeleteBinding 注册 DELETE 方法处理函数
func DeleteBinding(path string, fn interface{}, filters ...core.BeanSelector) *Mapping {
	return HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}

///////////////////// application /////////////////////////////

// HttpRequest 注册任意 HTTP 方法处理函数
func (app *application) HttpRequest(method uint32, path string, fn web.Handler, filters ...core.BeanSelector) *application {
	app.webMapping.HandleRequest(method, path, fn, filters)
	return app
}

// HttpMapping 注册任意 HTTP 方法处理函数
func (app *application) HttpMapping(method uint32, path string, fn web.HandlerFunc, filters ...core.BeanSelector) *application {
	app.webMapping.HandleRequest(method, path, web.FUNC(fn), filters)
	return app
}

// HttpBinding 注册任意 HTTP 方法处理函数
func (app *application) HttpBinding(method uint32, path string, fn interface{}, filters ...core.BeanSelector) *application {
	app.webMapping.HandleRequest(method, path, web.BIND(fn), filters)
	return app
}
