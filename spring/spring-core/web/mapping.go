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

package web

import (
	"fmt"
	"net/http"

	"github.com/go-spring/spring-core/util"
)

const (
	MethodGet     = 0x0001 // "GET"
	MethodHead    = 0x0002 // "HEAD"
	MethodPost    = 0x0004 // "POST"
	MethodPut     = 0x0008 // "PUT"
	MethodPatch   = 0x0010 // "PATCH"
	MethodDelete  = 0x0020 // "DELETE"
	MethodConnect = 0x0040 // "CONNECT"
	MethodOptions = 0x0080 // "OPTIONS"
	MethodTrace   = 0x0100 // "TRACE"
	MethodAny     = 0xffff
	MethodGetPost = MethodGet | MethodPost
)

var httpMethods = map[uint32]string{
	MethodGet:     http.MethodGet,
	MethodHead:    http.MethodHead,
	MethodPost:    http.MethodPost,
	MethodPut:     http.MethodPut,
	MethodPatch:   http.MethodPatch,
	MethodDelete:  http.MethodDelete,
	MethodConnect: http.MethodConnect,
	MethodOptions: http.MethodOptions,
	MethodTrace:   http.MethodTrace,
}

// GetMethod 返回 method 对应的 HTTP 方法
func GetMethod(method uint32) (r []string) {
	for k, v := range httpMethods {
		if method&k == k {
			r = append(r, v)
		}
	}
	return
}

// Mapper 路由映射器
type Mapper struct {
	method  uint32    // 方法
	path    string    // 路径
	handler Handler   // 处理函数
	filters []Filter  // 过滤器列表
	op      Operation // 描述文档
}

// NewMapper Mapper 的构造函数
func NewMapper(method uint32, path string, fn Handler, filters []Filter) *Mapper {
	return &Mapper{method: method, path: path, handler: fn, filters: filters}
}

// Key 返回 Mapper 的标识符
func (m *Mapper) Key() string {
	return fmt.Sprintf("0x%.4x@%s", m.method, m.path)
}

// Method 返回 Mapper 的方法
func (m *Mapper) Method() uint32 { return m.method }

// Path 返回 Mapper 的路径
func (m *Mapper) Path() string { return m.path }

// Handler 返回 Mapper 的处理函数
func (m *Mapper) Handler() Handler { return m.handler }

// Filters 返回 Mapper 的过滤器列表
func (m *Mapper) Filters() []Filter { return m.filters }

// Operation 设置与 Mapper 绑定的 Operation 对象
func (m *Mapper) Operation(op Operation) { m.op = op }

// Mapping 路由注册接口
type Mapping interface {

	// Route 返回和 Mapping 绑定的路由分组
	Route(basePath string, filters ...Filter) *Router

	// HandleRequest 注册任意 HTTP 方法处理函数
	HandleRequest(method uint32, path string, fn Handler, filters ...Filter) *Mapper

	// RequestMapping 注册任意 HTTP 方法处理函数
	RequestMapping(method uint32, path string, fn HandlerFunc, filters ...Filter) *Mapper

	// RequestBinding 注册任意 HTTP 方法处理函数
	RequestBinding(method uint32, path string, fn interface{}, filters ...Filter) *Mapper

	// HandleGet 注册 GET 方法处理函数
	HandleGet(path string, fn Handler, filters ...Filter) *Mapper

	// GetMapping 注册 GET 方法处理函数
	GetMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

	// GetBinding 注册 GET 方法处理函数
	GetBinding(path string, fn interface{}, filters ...Filter) *Mapper

	// HandlePost 注册 POST 方法处理函数
	HandlePost(path string, fn Handler, filters ...Filter) *Mapper

	// PostMapping 注册 POST 方法处理函数
	PostMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

	// PostBinding 注册 POST 方法处理函数
	PostBinding(path string, fn interface{}, filters ...Filter) *Mapper

	// HandlePut 注册 PUT 方法处理函数
	HandlePut(path string, fn Handler, filters ...Filter) *Mapper

	// PutMapping 注册 PUT 方法处理函数
	PutMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

	// PutBinding 注册 PUT 方法处理函数
	PutBinding(path string, fn interface{}, filters ...Filter) *Mapper

	// HandleDelete 注册 DELETE 方法处理函数
	HandleDelete(path string, fn Handler, filters ...Filter) *Mapper

	// DeleteMapping 注册 DELETE 方法处理函数
	DeleteMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper

	// DeleteBinding 注册 DELETE 方法处理函数
	DeleteBinding(path string, fn interface{}, filters ...Filter) *Mapper
}

// funcMapping 路由注册接口的默认实现
type funcMapping struct {
	request func(method uint32, path string, fn Handler, filters []Filter) *Mapper
}

// Route 返回和 Mapping 绑定的路由分组
func (r *funcMapping) Route(basePath string, filters ...Filter) *Router {
	panic(util.UnimplementedMethod)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *funcMapping) HandleRequest(method uint32, path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(method, path, fn, filters)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (r *funcMapping) RequestMapping(method uint32, path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(method, path, FUNC(fn), filters)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (r *funcMapping) RequestBinding(method uint32, path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(method, path, BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func (r *funcMapping) HandleGet(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, fn, filters)
}

// GetMapping 注册 GET 方法处理函数
func (r *funcMapping) GetMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, FUNC(fn), filters)
}

// GetBinding 注册 GET 方法处理函数
func (r *funcMapping) GetBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, BIND(fn), filters)
}

// HandlePost 注册 POST 方法处理函数
func (r *funcMapping) HandlePost(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, fn, filters)
}

// PostMapping 注册 POST 方法处理函数
func (r *funcMapping) PostMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, FUNC(fn), filters)
}

// PostBinding 注册 POST 方法处理函数
func (r *funcMapping) PostBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, BIND(fn), filters)
}

// HandlePut 注册 PUT 方法处理函数
func (r *funcMapping) HandlePut(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, fn, filters)
}

// PutMapping 注册 PUT 方法处理函数
func (r *funcMapping) PutMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, FUNC(fn), filters)
}

// PutBinding 注册 PUT 方法处理函数
func (r *funcMapping) PutBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, BIND(fn), filters)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *funcMapping) HandleDelete(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, fn, filters)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (r *funcMapping) DeleteMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, FUNC(fn), filters)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (r *funcMapping) DeleteBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, BIND(fn), filters)
}

// RootRouter 根路由，本质是 Mapper 的集合
type RootRouter interface {
	Mapping

	// AddMapper 添加一个 Mapper
	AddMapper(m *Mapper)

	// Mappers 返回映射器列表
	Mappers() map[string]*Mapper
}

// rootRouter 根路由表的默认实现
type rootRouter struct {
	Mapping

	mappers map[string]*Mapper
}

// NewRootRouter 返回一个 RootRouter 对象
func NewRootRouter() RootRouter {
	m := &rootRouter{mappers: make(map[string]*Mapper)}
	m.Mapping = &funcMapping{request: m.request}
	return m
}

// AddMapper 添加一个 Mapper
func (w *rootRouter) AddMapper(m *Mapper) { w.mappers[m.Key()] = m }

// Mappers 返回映射器列表
func (w *rootRouter) Mappers() map[string]*Mapper { return w.mappers }

// Route 返回和 Mapping 绑定的路由分组
func (w *rootRouter) Route(basePath string, filters ...Filter) *Router {
	return NewRouter(w, basePath, filters)
}

func (w *rootRouter) request(method uint32, path string, fn Handler, filters []Filter) *Mapper {
	m := NewMapper(method, path, fn, filters)
	w.mappers[m.Key()] = m
	return m
}

// Router 路由分组
type Router struct {
	Mapping

	basePath   string
	filters    []Filter
	rootRouter RootRouter
}

// NewRouter Router 的构造函数
func NewRouter(rootRouter RootRouter, basePath string, filters []Filter) *Router {
	r := &Router{filters: filters, basePath: basePath, rootRouter: rootRouter}
	r.Mapping = &funcMapping{request: r.request}
	return r
}

func (r *Router) request(method uint32, path string, fn Handler, filters []Filter) *Mapper {
	filters = append(r.filters, filters...)
	return r.rootRouter.HandleRequest(method, r.basePath+path, fn, filters...)
}

// Route 返回和 Mapping 绑定的路由分组
func (r *Router) Route(basePath string, filters ...Filter) *Router {
	filters = append(r.filters, filters...)
	return NewRouter(r.rootRouter, r.basePath+basePath, filters)
}
