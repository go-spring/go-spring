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
	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
)

// mapping Web 路由映射表
type mapping map[string]*web.Mapper

func (m mapping) addMapper(mapper *web.Mapper) *web.Mapper {
	m[mapper.Key()] = mapper
	return mapper
}

// HandleRequest 路由注册
func (m mapping) HandleRequest(method uint32, path string, fn web.Handler, filters []bean.Selector) *web.Mapper {
	var beanFilters []web.Filter
	for _, filter := range filters {
		beanFilters = append(beanFilters, &BeanFilter{filter})
	}
	return m.addMapper(web.NewMapper(method, path, fn, beanFilters))
}

type BeanFilter struct{ Filter bean.Selector }

func (f *BeanFilter) Invoke(ctx web.Context, chain web.FilterChain) {
	panic(util.UnimplementedMethod)
}

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
func (r *router) HandleRequest(method uint32, path string, fn web.Handler, filters ...bean.Selector) *web.Mapper {
	filters = append(r.filters, filters...) // 组合 router 和 mapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters)
}

// MappingRequest 注册任意 HTTP 方法处理函数
func (r *router) MappingRequest(method uint32, path string, fn web.HandlerFunc, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(method, path, web.FUNC(fn), filters...)
}

// BindingRequest 注册任意 HTTP 方法处理函数
func (r *router) BindingRequest(method uint32, path string, fn interface{}, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(method, path, web.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *router) HandleGet(path string, fn web.Handler, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, fn, filters...)
}

// MappingGet 注册 GET 方法处理函数
func (r *router) MappingGet(path string, fn web.HandlerFunc, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, web.FUNC(fn), filters...)
}

// BindingGet 注册 GET 方法处理函数
func (r *router) BindingGet(path string, fn interface{}, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodGet, path, web.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *router) HandlePost(path string, fn web.Handler, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, fn, filters...)
}

// MappingPost 注册 POST 方法处理函数
func (r *router) MappingPost(path string, fn web.HandlerFunc, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, web.FUNC(fn), filters...)
}

// BindingPost 注册 POST 方法处理函数
func (r *router) BindingPost(path string, fn interface{}, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPost, path, web.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *router) HandlePut(path string, fn web.Handler, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, fn, filters...)
}

// MappingPut 注册 PUT 方法处理函数
func (r *router) MappingPut(path string, fn web.HandlerFunc, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, web.FUNC(fn), filters...)
}

// BindingPut 注册 PUT 方法处理函数
func (r *router) BindingPut(path string, fn interface{}, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodPut, path, web.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *router) HandleDelete(path string, fn web.Handler, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, fn, filters...)
}

// MappingDelete 注册 DELETE 方法处理函数
func (r *router) MappingDelete(path string, fn web.HandlerFunc, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, web.FUNC(fn), filters...)
}

// BindingDelete 注册 DELETE 方法处理函数
func (r *router) BindingDelete(path string, fn interface{}, filters ...bean.Selector) *web.Mapper {
	return r.HandleRequest(web.MethodDelete, path, web.BIND(fn), filters...)
}
