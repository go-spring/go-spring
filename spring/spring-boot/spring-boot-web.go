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

package SpringBoot

import (
	"errors"
	"fmt"

	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-web"
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
func (m *WebMapping) HandleRequest(method uint32, path string, fn SpringWeb.Handler, filters []SpringWeb.Filter) *Mapping {
	mapping := newMapping(method, path, fn, filters)
	m.Mappings[mapping.Key()] = mapping
	return mapping
}

// Mapping 封装 Web 路由映射
type Mapping struct {
	handler SpringWeb.Handler
	mapper  *SpringWeb.Mapper    // 路由映射器
	cond    SpringCore.Condition // 判断条件
}

// newMapping Mapping 的构造函数
func newMapping(method uint32, path string, handler SpringWeb.Handler, filters []SpringWeb.Filter) *Mapping {
	return &Mapping{mapper: SpringWeb.NewMapper(method, path, nil, filters), handler: handler}
}

// Mapper 返回封装的 Mapper 对象
func (m *Mapping) Mapper() *SpringWeb.Mapper {
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
func (m *Mapping) Handler() SpringWeb.Handler {
	return m.handler
}

// Filters 返回 Mapper 的过滤器列表
func (m *Mapping) Filters() []SpringWeb.Filter {
	return m.mapper.Filters()
}

// WithCondition 设置一个 Condition
func (m *Mapping) WithCondition(cond SpringCore.Condition) *Mapping {
	m.cond = cond
	return m
}

// CheckCondition 成功返回 true，失败返回 false
func (m *Mapping) CheckCondition(ctx SpringCore.ApplicationContext) bool {
	if m.cond == nil {
		return true
	}
	return m.cond.Matches(ctx)
}

// Swagger 生成并返回 Swagger 操作节点
func (m *Mapping) Swagger() *SpringWeb.Operation {
	return m.mapper.Swagger("")
}

// Router 路由分组
type Router struct {
	mapping  *WebMapping
	basePath string
	filters  []SpringWeb.Filter
	cond     SpringCore.Condition // 判断条件
}

// newRouter Router 的构造函数
func newRouter(mapping *WebMapping, basePath string, filters []SpringWeb.Filter) *Router {
	return &Router{mapping: mapping, basePath: basePath, filters: filters}
}

// Route 创建子路由分组
func (r *Router) Route(basePath string, filters ...SpringWeb.Filter) *Router {
	return &Router{
		mapping:  r.mapping,
		basePath: r.basePath + basePath,
		filters:  append(r.filters, filters...),
		cond:     r.cond,
	}
}

// WithCondition 设置一个 Condition
func (r *Router) WithCondition(cond SpringCore.Condition) *Router {
	r.cond = cond
	return r
}

// HandleRequest 注册任意 HTTP 方法处理函数
func (r *Router) HandleRequest(method uint32, path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	filters = append(r.filters, filters...) // 组合 Router 和 Mapper 的过滤器列表
	return r.mapping.HandleRequest(method, r.basePath+path, fn, filters).WithCondition(r.cond)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func (r *Router) RequestMapping(method uint32, path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(method, path, SpringWeb.FUNC(fn), filters...)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func (r *Router) RequestBinding(method uint32, path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(method, path, SpringWeb.BIND(fn), filters...)
}

// HandleGet 注册 GET 方法处理函数
func (r *Router) HandleGet(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodGet, path, fn, filters...)
}

// GetMapping 注册 GET 方法处理函数
func (r *Router) GetMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodGet, path, SpringWeb.FUNC(fn), filters...)
}

// GetBinding 注册 GET 方法处理函数
func (r *Router) GetBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodGet, path, SpringWeb.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func (r *Router) HandlePost(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPost, path, fn, filters...)
}

// PostMapping 注册 POST 方法处理函数
func (r *Router) PostMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPost, path, SpringWeb.FUNC(fn), filters...)
}

// PostBinding 注册 POST 方法处理函数
func (r *Router) PostBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPost, path, SpringWeb.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func (r *Router) HandlePut(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPut, path, fn, filters...)
}

// PutMapping 注册 PUT 方法处理函数
func (r *Router) PutMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPut, path, SpringWeb.FUNC(fn), filters...)
}

// PutBinding 注册 PUT 方法处理函数
func (r *Router) PutBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodPut, path, SpringWeb.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func (r *Router) HandleDelete(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodDelete, path, fn, filters...)
}

// DeleteMapping 注册 DELETE 方法处理函数
func (r *Router) DeleteMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodDelete, path, SpringWeb.FUNC(fn), filters...)
}

// DeleteBinding 注册 DELETE 方法处理函数
func (r *Router) DeleteBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return r.HandleRequest(SpringWeb.MethodDelete, path, SpringWeb.BIND(fn), filters...)
}

///////////////////// 全局函数 /////////////////////////////

// DefaultWebMapping 默认的 Web 路由映射表
var DefaultWebMapping = NewWebMapping()

// Route 返回和 Mapping 绑定的路由分组
func Route(basePath string, filters ...SpringWeb.Filter) *Router {
	return newRouter(DefaultWebMapping, basePath, filters)
}

// HandleRequest 注册任意 HTTP 方法处理函数
func HandleRequest(method uint32, path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return DefaultWebMapping.HandleRequest(method, path, fn, filters)
}

// RequestMapping 注册任意 HTTP 方法处理函数
func RequestMapping(method uint32, path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return DefaultWebMapping.HandleRequest(method, path, SpringWeb.FUNC(fn), filters)
}

// RequestBinding 注册任意 HTTP 方法处理函数
func RequestBinding(method uint32, path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return DefaultWebMapping.HandleRequest(method, path, SpringWeb.BIND(fn), filters)
}

// HandleGet 注册 GET 方法处理函数
func HandleGet(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodGet, path, fn, filters...)
}

// GetMapping 注册 GET 方法处理函数
func GetMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodGet, path, SpringWeb.FUNC(fn), filters...)
}

// GetBinding 注册 GET 方法处理函数
func GetBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodGet, path, SpringWeb.BIND(fn), filters...)
}

// HandlePost 注册 POST 方法处理函数
func HandlePost(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPost, path, fn, filters...)
}

// PostMapping 注册 POST 方法处理函数
func PostMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPost, path, SpringWeb.FUNC(fn), filters...)
}

// PostBinding 注册 POST 方法处理函数
func PostBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPost, path, SpringWeb.BIND(fn), filters...)
}

// HandlePut 注册 PUT 方法处理函数
func HandlePut(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPut, path, fn, filters...)
}

// PutMapping 注册 PUT 方法处理函数
func PutMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPut, path, SpringWeb.FUNC(fn), filters...)
}

// PutBinding 注册 PUT 方法处理函数
func PutBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodPut, path, SpringWeb.BIND(fn), filters...)
}

// HandleDelete 注册 DELETE 方法处理函数
func HandleDelete(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodDelete, path, fn, filters...)
}

// DeleteMapping 注册 DELETE 方法处理函数
func DeleteMapping(path string, fn SpringWeb.HandlerFunc, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodDelete, path, SpringWeb.FUNC(fn), filters...)
}

// DeleteBinding 注册 DELETE 方法处理函数
func DeleteBinding(path string, fn interface{}, filters ...SpringWeb.Filter) *Mapping {
	return HandleRequest(SpringWeb.MethodDelete, path, SpringWeb.BIND(fn), filters...)
}

///////////////////// Web Filter //////////////////////

// WebFilterArray 首字母小写太难看，因此不管它是否真正需要公开
type WebFilterArray interface {
	Get(ctx SpringCore.ApplicationContext) []SpringWeb.Filter
}

// WebFilterArray 首字母小写太难看，因此不管它是否真正需要公开
type WebFilterArrayImpl struct {
	filters []SpringWeb.Filter
}

func (l *WebFilterArrayImpl) Get(ctx SpringCore.ApplicationContext) []SpringWeb.Filter {
	return l.filters
}

// WebFilterArray 首字母小写太难看，因此不管它是否真正需要公开
type WebFilterBeanArrayImpl struct {
	beans []SpringCore.BeanSelector
}

func (l *WebFilterBeanArrayImpl) Get(ctx SpringCore.ApplicationContext) []SpringWeb.Filter {
	var result []SpringWeb.Filter
	for _, beanId := range l.beans {
		var filter SpringWeb.Filter
		if !ctx.GetBean(&filter, beanId) {
			panic(fmt.Errorf("can't get filter %v", beanId))
		}
		result = append(result, filter)
	}
	return result
}

// ConditionalWebFilter 为 SpringWeb.Filter 增加一个判断条件
type ConditionalWebFilter struct {
	cond SpringCore.Condition // 判断条件
	list WebFilterArray
}

// Filter 封装一个 SpringWeb.Filter 对象
func Filter(filters ...SpringWeb.Filter) *ConditionalWebFilter {
	return &ConditionalWebFilter{list: &WebFilterArrayImpl{filters}}
}

// FilterBean 封装一个 Bean 选择器
func FilterBean(selectors ...SpringCore.BeanSelector) *ConditionalWebFilter {
	return &ConditionalWebFilter{list: &WebFilterBeanArrayImpl{selectors}}
}

func (f *ConditionalWebFilter) Invoke(ctx SpringWeb.Context, chain SpringWeb.FilterChain) {
	panic(errors.New("shouldn't call this method"))
}

// WithCondition 设置一个 Condition
func (f *ConditionalWebFilter) WithCondition(cond SpringCore.Condition) *ConditionalWebFilter {
	f.cond = cond
	return f
}

func (f *ConditionalWebFilter) ResolveFilters(ctx SpringCore.ApplicationContext) []SpringWeb.Filter {
	if f.cond != nil && f.cond.Matches(ctx) {
		return f.list.Get(ctx)
	}
	return nil
}
