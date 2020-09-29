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

// Router 路由分组
type Router struct {
	UrlRegister

	mapping  WebMapping
	basePath string
	filters  []Filter
}

// NewRouter Router 的构造函数，不依赖具体的 WebMapping 对象
func NewRouter(basePath string, filters ...Filter) *Router {
	return routerWithMapping(NewDefaultWebMapping(), basePath, filters)
}

// routerWithMapping Router 的构造函数，依赖具体的 WebMapping 对象
func routerWithMapping(mapping WebMapping, basePath string, filters []Filter) *Router {
	r := &Router{}
	r.filters = filters
	r.mapping = mapping
	r.basePath = basePath
	r.UrlRegister = &defaultUrlRegister{request: r.request}
	return r
}

func (r *Router) request(method uint32, path string, fn Handler, filters []Filter) *Mapper {
	filters = append(r.filters, filters...)
	return r.mapping.Request(method, r.basePath+path, fn, filters...)
}

// Route 返回和 Mapping 绑定的路由分组
func (r *Router) Route(basePath string, filters ...Filter) *Router {
	filters = append(r.filters, filters...)
	return routerWithMapping(r.mapping, r.basePath+basePath, filters)
}
