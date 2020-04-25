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
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-core"
)

// WebMapping Web 路由映射表
type WebMapping struct {
	Mappings map[string]*Mapping
}

// NewWebMapping WebMapping 的构造函数
func NewWebMapping() *WebMapping {
	return &WebMapping{
		Mappings: make(map[string]*Mapping),
	}
}

// Request
func (m *WebMapping) Request(method uint32, path string, fn SpringWeb.Handler) *Mapping {
	mapping := NewMapping(SpringWeb.NewMapper(method, path, fn, nil))
	m.Mappings[mapping.Key()] = mapping
	return mapping
}

// Mapping 封装 Web 路由映射
type Mapping struct {
	mapper      *SpringWeb.Mapper       // 路由映射器
	ports       []int                   // 路由期望的端口
	filterNames []string                // 过滤器列表
	cond        *SpringCore.Conditional // 判断条件
}

// NewMapping Mapping 的构造函数
func NewMapping(mapper *SpringWeb.Mapper) *Mapping {
	return &Mapping{
		mapper: mapper,
		cond:   SpringCore.NewConditional(),
	}
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

// Handler 返回 Mapper 的处理函数
func (m *Mapping) Handler() SpringWeb.Handler {
	return m.mapper.Handler()
}

// Filters 返回 Mapper 的过滤器列表
func (m *Mapping) Filters() []SpringWeb.Filter {
	return m.mapper.Filters()
}

// Filters 设置 Mapper 的过滤器列表
func (m *Mapping) SetFilters(filters ...SpringWeb.Filter) *Mapping {
	m.mapper.SetFilters(filters)
	return m
}

// Ports 返回路由期望的端口
func (m *Mapping) Ports() []int {
	return m.ports
}

// OnPorts 设置路由期望的端口
func (m *Mapping) OnPorts(ports ...int) *Mapping {
	m.ports = ports
	return m
}

// FilterNames 返回过滤器列表
func (m *Mapping) FilterNames() []string {
	return m.filterNames
}

// SetFilterNames 设置过滤器列表
func (m *Mapping) SetFilterNames(filterNames ...string) *Mapping {
	m.filterNames = append(m.filterNames, filterNames...)
	return m
}

// Or c=a||b
func (m *Mapping) Or() *Mapping {
	m.cond.Or()
	return m
}

// And c=a&&b
func (m *Mapping) And() *Mapping {
	m.cond.And()
	return m
}

// ConditionOn 设置一个 Condition
func (m *Mapping) ConditionOn(cond SpringCore.Condition) *Mapping {
	m.cond.OnCondition(cond)
	return m
}

// ConditionNot 设置一个取反的 Condition
func (m *Mapping) ConditionNot(cond SpringCore.Condition) *Mapping {
	m.cond.OnConditionNot(cond)
	return m
}

// ConditionOnProperty 设置一个 PropertyCondition
func (m *Mapping) ConditionOnProperty(name string) *Mapping {
	m.cond.OnProperty(name)
	return m
}

// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
func (m *Mapping) ConditionOnMissingProperty(name string) *Mapping {
	m.cond.OnMissingProperty(name)
	return m
}

// ConditionOnPropertyValue 设置一个 PropertyValueCondition
func (m *Mapping) ConditionOnPropertyValue(name string, havingValue interface{}) *Mapping {
	m.cond.OnPropertyValue(name, havingValue)
	return m
}

// ConditionOnBean 设置一个 BeanCondition
func (m *Mapping) ConditionOnBean(selector interface{}) *Mapping {
	m.cond.OnBean(selector)
	return m
}

// ConditionOnMissingBean 设置一个 MissingBeanCondition
func (m *Mapping) ConditionOnMissingBean(selector interface{}) *Mapping {
	m.cond.OnMissingBean(selector)
	return m
}

// ConditionOnExpression 设置一个 ExpressionCondition
func (m *Mapping) ConditionOnExpression(expression string) *Mapping {
	m.cond.OnExpression(expression)
	return m
}

// ConditionOnMatches 设置一个 FunctionCondition
func (m *Mapping) ConditionOnMatches(fn SpringCore.ConditionFunc) *Mapping {
	m.cond.OnMatches(fn)
	return m
}

// ConditionOnProfile 设置一个 ProfileCondition
func (m *Mapping) ConditionOnProfile(profile string) *Mapping {
	m.cond.OnProfile(profile)
	return m
}

// Matches 成功返回 true，失败返回 false
func (m *Mapping) Matches(ctx SpringCore.SpringContext) bool {
	return m.cond.Matches(ctx)
}

// Swagger 生成并返回 Swagger 操作节点
func (m *Mapping) Swagger() *SpringWeb.Operation {
	return m.mapper.Swagger("")
}

// Router 路由分组
type Router struct {
	mapping     *WebMapping
	basePath    string
	filters     []SpringWeb.Filter
	ports       []int                   // 路由期望的端口
	filterNames []string                // 过滤器列表
	cond        *SpringCore.Conditional // 判断条件
}

// NewRouter Router 的构造函数
func NewRouter(mapping *WebMapping, basePath string) *Router {
	return &Router{
		mapping:  mapping,
		basePath: basePath,
		cond:     SpringCore.NewConditional(),
	}
}

// Filters 设置过滤器列表
func (r *Router) SetFilters(filters ...SpringWeb.Filter) *Router {
	r.filters = filters
	return r
}

// OnPorts 设置路由期望的端口
func (r *Router) OnPorts(ports ...int) *Router {
	r.ports = ports
	return r
}

// SetFilterNames 设置过滤器列表
func (r *Router) SetFilterNames(filterNames ...string) *Router {
	r.filterNames = filterNames
	return r
}

// Or c=a||b
func (r *Router) Or() *Router {
	r.cond.Or()
	return r
}

// And c=a&&b
func (r *Router) And() *Router {
	r.cond.And()
	return r
}

// ConditionOn 设置一个 Condition
func (r *Router) ConditionOn(cond SpringCore.Condition) *Router {
	r.cond.OnCondition(cond)
	return r
}

// ConditionNot 设置一个取反的 Condition
func (r *Router) ConditionNot(cond SpringCore.Condition) *Router {
	r.cond.OnConditionNot(cond)
	return r
}

// ConditionOnProperty 设置一个 PropertyCondition
func (r *Router) ConditionOnProperty(name string) *Router {
	r.cond.OnProperty(name)
	return r
}

// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
func (r *Router) ConditionOnMissingProperty(name string) *Router {
	r.cond.OnMissingProperty(name)
	return r
}

// ConditionOnPropertyValue 设置一个 PropertyValueCondition
func (r *Router) ConditionOnPropertyValue(name string, havingValue interface{}) *Router {
	r.cond.OnPropertyValue(name, havingValue)
	return r
}

// ConditionOnBean 设置一个 BeanCondition
func (r *Router) ConditionOnBean(selector interface{}) *Router {
	r.cond.OnBean(selector)
	return r
}

// ConditionOnMissingBean 设置一个 MissingBeanCondition
func (r *Router) ConditionOnMissingBean(selector interface{}) *Router {
	r.cond.OnMissingBean(selector)
	return r
}

// ConditionOnExpression 设置一个 ExpressionCondition
func (r *Router) ConditionOnExpression(expression string) *Router {
	r.cond.OnExpression(expression)
	return r
}

// ConditionOnMatches 设置一个 FunctionCondition
func (r *Router) ConditionOnMatches(fn SpringCore.ConditionFunc) *Router {
	r.cond.OnMatches(fn)
	return r
}

// ConditionOnProfile 设置一个 ProfileCondition
func (r *Router) ConditionOnProfile(profile string) *Router {
	r.cond.OnProfile(profile)
	return r
}

// Request 注册任意 HTTP 方法处理函数
func (r *Router) Request(method uint32, path string, fn SpringWeb.Handler) *Mapping {
	return r.mapping.Request(method, r.basePath+path, fn).
		OnPorts(r.ports...).SetFilters(r.filters...).
		SetFilterNames(r.filterNames...).
		ConditionOn(r.cond)
}

// GET 注册 GET 方法处理函数
func (r *Router) GET(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodGet, path, fn)
}

// POST 注册 POST 方法处理函数
func (r *Router) POST(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodPost, path, fn)
}

// PATCH 注册 PATCH 方法处理函数
func (r *Router) PATCH(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodPatch, path, fn)
}

// PUT 注册 PUT 方法处理函数
func (r *Router) PUT(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodPut, path, fn)
}

// DELETE 注册 DELETE 方法处理函数
func (r *Router) DELETE(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodDelete, path, fn)
}

// HEAD 注册 HEAD 方法处理函数
func (r *Router) HEAD(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodHead, path, fn)
}

// OPTIONS 注册 OPTIONS 方法处理函数
func (r *Router) OPTIONS(path string, fn SpringWeb.Handler) *Mapping {
	return r.Request(SpringWeb.MethodOptions, path, fn)
}

///////////////////// 以下是全局函数 /////////////////////////////

// DefaultWebMapping 默认的 Web 路由映射表
var DefaultWebMapping = NewWebMapping()

// Route 返回和 Mapping 绑定的路由分组
func Route(basePath string) *Router {
	return NewRouter(DefaultWebMapping, basePath)
}

// RequestMapping
func RequestMapping(method uint32, path string, fn SpringWeb.Handler) *Mapping {
	return DefaultWebMapping.Request(method, path, fn)
}

// GetMapping
func GetMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping(SpringWeb.MethodGet, path, fn)
}

// PostMapping
func PostMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping(SpringWeb.MethodPost, path, fn)
}

// PutMapping
func PutMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping(SpringWeb.MethodPut, path, fn)
}

// PatchMapping
func PatchMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping(SpringWeb.MethodPatch, path, fn)
}

// DeleteMapping
func DeleteMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping(SpringWeb.MethodDelete, path, fn)
}
