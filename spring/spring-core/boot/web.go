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
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/web"
)

// mapping Web 路由映射表
type mapping map[string]*mapper

func (m mapping) addMapper(mapper *mapper) *mapper {
	m[mapper.Key()] = mapper
	return mapper
}

// HandleRequest 路由注册
func (m mapping) HandleRequest(method uint32, path string, fn web.Handler, filters []bean.Selector) *mapper {
	return m.addMapper(newMapper(method, path, fn, filters))
}

// mapper 封装 Web 路由映射
type mapper struct {
	mapper  *web.Mapper
	filters []bean.Selector
}

// newMapper mapper 的构造函数
func newMapper(method uint32, path string, handler web.Handler, filters ...bean.Selector) *mapper {
	return &mapper{mapper: web.NewMapper(method, path, handler, nil), filters: filters}
}

// Mapper 返回封装的 Mapper 对象
func (m *mapper) Mapper() *web.Mapper {
	return m.mapper
}

// Key 返回 mapper 的标识符
func (m *mapper) Key() string {
	return m.mapper.Key()
}

// Method 返回 mapper 的方法
func (m *mapper) Method() uint32 {
	return m.mapper.Method()
}

// Path 返回 mapper 的路径
func (m *mapper) Path() string {
	return m.mapper.Path()
}

// HandlerSelector 返回处理函数选择器
func (m *mapper) Handler() web.Handler {
	return m.mapper.Handler()
}

// Filters 返回 mapper 的过滤器列表
func (m *mapper) Filters() []bean.Selector {
	return m.filters
}

//// Swagger 生成并返回 Swagger 操作节点
//func (m *mapper) Swagger() *web.Operation {
//	return m.mapper.Swagger("")
//}

// router 路由分组
type router struct {
	mapping  mapping
	basePath string
	filters  []bean.Selector
}

// newRouter router 的构造函数
func newRouter(mapping mapping, basePath string, filters ...bean.Selector) *router {
	return &router{mapping: mapping, basePath: basePath, filters: filters}
}

// Route 创建子路由分组
func (r *router) Route(basePath string, filters ...bean.Selector) *router {
	return &router{
		mapping:  r.mapping,
		basePath: r.basePath + basePath,
		filters:  append(r.filters, filters...),
	}
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *router) HandleRequest(method uint32, path string, fn web.Handler, filters ...bean.Selector) *mapper {
	filters = append(r.filters, filters...) // 组合 router 和 mapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func (r *router) MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...bean.Selector) *mapper {
	return r.HandleRequest(method, path, web.FUNC(fn), filters...)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func (r *router) BindingRequest(method uint32, path string, fn interface{}, filters ...bean.Selector) *mapper {
	return r.HandleRequest(method, path, web.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *router) HandleGet(path string, fn web.Handler, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func (r *router) MappingGet(path string, fn web.HandlerFunc, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func (r *router) BindingGet(path string, fn interface{}, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *router) HandlePost(path string, fn web.Handler, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func (r *router) MappingPost(path string, fn web.HandlerFunc, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func (r *router) BindingPost(path string, fn interface{}, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *router) HandlePut(path string, fn web.Handler, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func (r *router) MappingPut(path string, fn web.HandlerFunc, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func (r *router) BindingPut(path string, fn interface{}, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *router) HandleDelete(path string, fn web.Handler, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func (r *router) MappingDelete(path string, fn web.HandlerFunc, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func (r *router) BindingDelete(path string, fn interface{}, filters ...bean.Selector) *mapper {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}
