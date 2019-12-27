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

// UrlMapping
var UrlMapping = make(map[string]*Mapping)

// Mapping
type Mapping struct {
	SpringWeb.Mapper

	filterNames []string // 过滤器列表

	ports []int  // 允许注册到哪些端口上
	doc   string // 接口的说明文档

	*SpringCore.Conditional // 判断条件
}

// 构造函数
func NewMapping(mapper SpringWeb.Mapper) *Mapping {
	return &Mapping{
		Mapper:      mapper,
		Conditional: SpringCore.NewConditional(),
	}
}

// Ports
func (m *Mapping) Ports() []int {
	return m.ports
}

// SetPorts
func (m *Mapping) SetPorts(ports []int) {
	m.ports = ports
}

// Doc
func (m *Mapping) Doc() string {
	return m.doc
}

// SetDoc
func (m *Mapping) SetDoc(doc string) {
	m.doc = doc
}

// FilterNames
func (m *Mapping) FilterNames() []string {
	return m.filterNames
}

// SetFilterNames
func (m *Mapping) SetFilterNames(filterNames ...string) {
	m.filterNames = filterNames
}

// Or c=a||b
func (m *Mapping) Or() *Mapping {
	m.Conditional.Or()
	return m
}

// And c=a&&b
func (m *Mapping) And() *Mapping {
	m.Conditional.And()
	return m
}

// 设置一个 Condition
func (m *Mapping) ConditionOn(cond SpringCore.Condition) *Mapping {
	m.Conditional.OnCondition(cond)
	return m
}

// 设置一个 PropertyCondition
func (m *Mapping) ConditionOnProperty(name string) *Mapping {
	m.Conditional.OnProperty(name)
	return m
}

// 设置一个 MissingPropertyCondition
func (m *Mapping) ConditionOnMissingProperty(name string) *Mapping {
	m.Conditional.OnMissingProperty(name)
	return m
}

// 设置一个 PropertyValueCondition
func (m *Mapping) ConditionOnPropertyValue(name string, havingValue interface{}) *Mapping {
	m.Conditional.OnPropertyValue(name, havingValue)
	return m
}

// 设置一个 BeanCondition
func (m *Mapping) ConditionOnBean(beanId string) *Mapping {
	m.Conditional.OnBean(beanId)
	return m
}

// 设置一个 MissingBeanCondition
func (m *Mapping) ConditionOnMissingBean(beanId string) *Mapping {
	m.Conditional.OnMissingBean(beanId)
	return m
}

// 设置一个 ExpressionCondition
func (m *Mapping) ConditionOnExpression(expression string) *Mapping {
	m.Conditional.OnExpression(expression)
	return m
}

// 设置一个 FunctionCondition
func (m *Mapping) ConditionOnMatches(fn SpringCore.ConditionFunc) *Mapping {
	m.Conditional.OnMatches(fn)
	return m
}

// ConditionOnProfile 设置一个 ProfileCondition
func (m *Mapping) ConditionOnProfile(profile string) *Mapping {
	m.Conditional.OnProfile(profile)
	return m
}

// RequestMapping
func RequestMapping(method string, path string, fn SpringWeb.Handler) *Mapping {
	mapper := SpringWeb.NewMapper(method, path, fn, nil)
	mapping := NewMapping(mapper)
	UrlMapping[path+method] = mapping
	return mapping
}

// GetMapping
func GetMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("GET", path, fn)
}

// PostMapping
func PostMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("POST", path, fn)
}

// PutMapping
func PutMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("PUT", path, fn)
}

// PatchMapping
func PatchMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("PATCH", path, fn)
}

// DeleteMapping
func DeleteMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("DELETE", path, fn)
}
