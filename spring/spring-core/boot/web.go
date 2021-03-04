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
	"github.com/go-spring/spring-core/web"
)

var Mapping = make(map[string]*web.Mapper)

func newMapper(method uint32, path string, fn web.Handler) *web.Mapper {
	mapper := web.NewMapper(method, path, fn, nil)
	Mapping[mapper.Key()] = mapper
	return mapper
}

// router 路由分组
type router struct{ basePath string }

// newRouter router 的构造函数
func newRouter(basePath string) *router {
	return &router{basePath: basePath}
}

// Route 创建子路由分组
func (r *router) Route(basePath string) *router {
	return newRouter(r.basePath + basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *router) HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return newMapper(method, r.basePath+path, fn)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func (r *router) MappingRequest(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return r.HandleRequest(method, path, web.FUNC(fn))
}

// BindingRequest 注册任意 HTTP 方法处理函数
func (r *router) BindingRequest(method uint32, path string, fn interface{}) *web.Mapper {
	return r.HandleRequest(method, path, web.BIND(fn))
}

// HandleGet 注册 GET 方法处理函数
func (r *router) HandleGet(path string, fn web.Handler) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, fn)
}

// MappingGet 注册 GET 方法处理函数
func (r *router) MappingGet(path string, fn web.HandlerFunc) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn))
}

// BindingGet 注册 GET 方法处理函数
func (r *router) BindingGet(path string, fn interface{}) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn))
}

// HandlePost 注册 POST 方法处理函数
func (r *router) HandlePost(path string, fn web.Handler) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, fn)
}

// MappingPost 注册 POST 方法处理函数
func (r *router) MappingPost(path string, fn web.HandlerFunc) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn))
}

// BindingPost 注册 POST 方法处理函数
func (r *router) BindingPost(path string, fn interface{}) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn))
}

// HandlePut 注册 PUT 方法处理函数
func (r *router) HandlePut(path string, fn web.Handler) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, fn)
}

// MappingPut 注册 PUT 方法处理函数
func (r *router) MappingPut(path string, fn web.HandlerFunc) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn))
}

// BindingPut 注册 PUT 方法处理函数
func (r *router) BindingPut(path string, fn interface{}) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn))
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *router) HandleDelete(path string, fn web.Handler) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, fn)
}

// MappingDelete 注册 DELETE 方法处理函数
func (r *router) MappingDelete(path string, fn web.HandlerFunc) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn))
}

// BindingDelete 注册 DELETE 方法处理函数
func (r *router) BindingDelete(path string, fn interface{}) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn))
}

// Route 返回和 app.mapper 绑定的路由分组
func Route(basePath string) *router {
	return newRouter(basePath)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn web.Handler) *web.Mapper {
	return newMapper(method, path, fn)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func MappingRequest(method uint32, path string, fn web.HandlerFunc) *web.Mapper {
	return newMapper(method, path, web.FUNC(fn))
}

// BindingRequest 注册任意 HTTP 方法处理函数
func BindingRequest(method uint32, path string, fn interface{}) *web.Mapper {
	return newMapper(method, path, web.BIND(fn))
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn web.Handler) *web.Mapper {
	return newMapper(web.MethodGet, path, fn)
}

// MappingGet 注册 GET 方法处理函数
func MappingGet(path string, fn web.HandlerFunc) *web.Mapper {
	return newMapper(web.MethodGet, path, web.FUNC(fn))
}

// BindingGet 注册 GET 方法处理函数
func BindingGet(path string, fn interface{}) *web.Mapper {
	return newMapper(web.MethodGet, path, web.BIND(fn))
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn web.Handler) *web.Mapper {
	return newMapper(web.MethodPost, path, fn)
}

// MappingPost 注册 POST 方法处理函数
func MappingPost(path string, fn web.HandlerFunc) *web.Mapper {
	return newMapper(web.MethodPost, path, web.FUNC(fn))
}

// BindingPost 注册 POST 方法处理函数
func BindingPost(path string, fn interface{}) *web.Mapper {
	return newMapper(web.MethodPost, path, web.BIND(fn))
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn web.Handler) *web.Mapper {
	return newMapper(web.MethodPut, path, fn)
}

// MappingPut 注册 PUT 方法处理函数
func MappingPut(path string, fn web.HandlerFunc) *web.Mapper {
	return newMapper(web.MethodPut, path, web.FUNC(fn))
}

// BindingPut 注册 PUT 方法处理函数
func BindingPut(path string, fn interface{}) *web.Mapper {
	return newMapper(web.MethodPut, path, web.BIND(fn))
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn web.Handler) *web.Mapper {
	return newMapper(web.MethodDelete, path, fn)
}

// MappingDelete 注册 DELETE 方法处理函数
func MappingDelete(path string, fn web.HandlerFunc) *web.Mapper {
	return newMapper(web.MethodDelete, path, web.FUNC(fn))
}

// BindingDelete 注册 DELETE 方法处理函数
func BindingDelete(path string, fn interface{}) *web.Mapper {
	return newMapper(web.MethodDelete, path, web.BIND(fn))
}
