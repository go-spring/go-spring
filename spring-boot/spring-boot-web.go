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

//
// UrlMapping
//
var UrlMapping = make(map[string]*Mapping)

//
// Mapping
//
type Mapping struct {
	mapper SpringWeb.Mapper

	ports []int  // 允许注册到哪些端口上
	doc   string // 接口的说明文档

	SpringCore.Constriction
}

//
// 构造函数
//
func NewMapping(mapper SpringWeb.Mapper) *Mapping {
	return &Mapping{
		mapper: mapper,
	}
}

//
// Mapper
//
func (m *Mapping) Mapper() SpringWeb.Mapper {
	return m.mapper
}

//
// Ports
//
func (m *Mapping) Ports() []int {
	return m.ports
}

//
// SetPorts
//
func (m *Mapping) SetPorts(ports []int) {
	m.ports = ports
}

//
// Doc
//
func (m *Mapping) Doc() string {
	return m.doc
}

//
// SetDoc
//
func (m *Mapping) SetDoc(doc string) {
	m.doc = doc
}

//
// 设置一个 Condition
//
func (m *Mapping) ConditionOn(cond SpringCore.Condition) *Mapping {
	m.Constriction.ConditionOn(cond)
	return m
}

//
// 设置一个 PropertyCondition
//
func (m *Mapping) ConditionOnProperty(name string) *Mapping {
	m.Constriction.ConditionOnProperty(name)
	return m
}

//
// 设置一个 MissingPropertyCondition
//
func (m *Mapping) ConditionOnMissingProperty(name string) *Mapping {
	m.Constriction.ConditionOnMissingProperty(name)
	return m
}

//
// 设置一个 PropertyValueCondition
//
func (m *Mapping) ConditionOnPropertyValue(name string, havingValue interface{}) *Mapping {
	m.Constriction.ConditionOnPropertyValue(name, havingValue)
	return m
}

//
// 设置一个 BeanCondition
//
func (m *Mapping) ConditionOnBean(beanId string) *Mapping {
	m.Constriction.ConditionOnBean(beanId)
	return m
}

//
// 设置一个 MissingBeanCondition
//
func (m *Mapping) ConditionOnMissingBean(beanId string) *Mapping {
	m.Constriction.ConditionOnMissingBean(beanId)
	return m
}

//
// 设置一个 ExpressionCondition
//
func (m *Mapping) ConditionOnExpression(expression string) *Mapping {
	m.Constriction.ConditionOnExpression(expression)
	return m
}

//
// 设置一个 FunctionCondition
//
func (m *Mapping) ConditionOnMatches(fn SpringCore.ConditionFunc) *Mapping {
	m.Constriction.ConditionOnMatches(fn)
	return m
}

//
// 设置 bean 的运行环境
//
func (m *Mapping) Profile(profile string) *Mapping {
	m.Constriction.Profile(profile)
	return m
}

//
// RequestMapping
//
func RequestMapping(method string, path string, fn SpringWeb.Handler) *Mapping {
	mapper := SpringWeb.NewMapper(method, path, fn, nil)
	mapping := NewMapping(mapper)
	UrlMapping[path+method] = mapping
	return mapping
}

//
// GetMapping
//
func GetMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("GET", path, fn)
}

//
// PostMapping
//
func PostMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("POST", path, fn)
}

//
// PutMapping
//
func PutMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("PUT", path, fn)
}

//
// PatchMapping
//
func PatchMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("PATCH", path, fn)
}

//
// DeleteMapping
//
func DeleteMapping(path string, fn SpringWeb.Handler) *Mapping {
	return RequestMapping("DELETE", path, fn)
}
