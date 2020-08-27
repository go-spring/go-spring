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

package SpringWeb

// UrlRegister 路由注册接口
type UrlRegister interface {

	// Request 注册任意 HTTP 方法处理函数
	Request(method uint32, path string, fn Handler, filters ...Filter) *Mapper

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

// defaultUrlRegister 路由注册接口的默认实现
type defaultUrlRegister struct {
	request func(method uint32, path string, fn Handler, filters []Filter) *Mapper
}

// Request 注册任意 HTTP 方法处理函数
func (r *defaultUrlRegister) Request(method uint32, path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(method, path, fn, filters)
}

// HandleGet 注册 GET 方法处理函数
func (r *defaultUrlRegister) HandleGet(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, fn, filters)
}

// GetMapping 注册 GET 方法处理函数
func (r *defaultUrlRegister) GetMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, FUNC(fn), filters)
}

// GetBinding 注册 GET 方法处理函数
func (r *defaultUrlRegister) GetBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodGet, path, BIND(fn), filters)
}

// HandlePost 注册 POST 方法处理函数
func (r *defaultUrlRegister) HandlePost(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, fn, filters)
}

// PostMapping 注册 POST 方法处理函数
func (r *defaultUrlRegister) PostMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, FUNC(fn), filters)
}

// PostBinding 注册 POST 方法处理函数
func (r *defaultUrlRegister) PostBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodPost, path, BIND(fn), filters)
}

// HandlePut 注册 PUT 方法处理函数
func (r *defaultUrlRegister) HandlePut(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, fn, filters)
}

// PutMapping 注册 PUT 方法处理函数
func (r *defaultUrlRegister) PutMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, FUNC(fn), filters)
}

// PutBinding 注册 PUT 方法处理函数
func (r *defaultUrlRegister) PutBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodPut, path, BIND(fn), filters)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *defaultUrlRegister) HandleDelete(path string, fn Handler, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, fn, filters)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (r *defaultUrlRegister) DeleteMapping(path string, fn HandlerFunc, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, FUNC(fn), filters)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (r *defaultUrlRegister) DeleteBinding(path string, fn interface{}, filters ...Filter) *Mapper {
	return r.request(MethodDelete, path, BIND(fn), filters)
}

// WebMapping 路由表，Spring-Web 使用的路由规则和 echo 完全相同，并对 gin 做了适配。
type WebMapping interface {
	UrlRegister

	// Mappers 返回映射器列表
	Mappers() map[string]*Mapper

	// AddMapper 添加一个 Mapper
	AddMapper(m *Mapper) *Mapper

	// Route 返回和 Mapping 绑定的路由分组
	Route(basePath string, filters ...Filter) *Router
}

// defaultWebMapping 路由表的默认实现
type defaultWebMapping struct {
	UrlRegister

	mappers map[string]*Mapper
}

// NewDefaultWebMapping defaultWebMapping 的构造函数
func NewDefaultWebMapping() *defaultWebMapping {
	m := &defaultWebMapping{}
	m.mappers = make(map[string]*Mapper)
	m.UrlRegister = &defaultUrlRegister{request: m.request}
	return m
}

// Mappers 返回映射器列表
func (w *defaultWebMapping) Mappers() map[string]*Mapper {
	return w.mappers
}

// AddMapper 添加一个 Mapper
func (w *defaultWebMapping) AddMapper(m *Mapper) *Mapper {
	w.mappers[m.Key()] = m
	return m
}

// Route 返回和 Mapping 绑定的路由分组
func (w *defaultWebMapping) Route(basePath string, filters ...Filter) *Router {
	return routerWithMapping(w, basePath, filters)
}

func (w *defaultWebMapping) request(method uint32, path string, fn Handler, filters []Filter) *Mapper {
	m := NewMapper(method, path, fn, filters)
	w.mappers[m.Key()] = m
	return m
}
