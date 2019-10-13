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

package SpringCore

import (
	"reflect"
)

//
// 定义 SpringBean 类型
//
type SpringBean interface{}

//
// SpringBean 初始化接口
//
type SpringBeanInitialization interface {
	InitBean(ctx SpringContext)
}

//
// 定义 SpringBeanDefinition 类型
//
type SpringBeanDefinition struct {
	Bean  SpringBean
	Name  string
	Init  int
	Type  reflect.Type
	Value reflect.Value
}

//
// 定义 SpringContext 接口
//
type SpringContext interface {
	// 使用默认的名称注册 SpringBean 对象
	RegisterBean(bean SpringBean)

	// 使用指定的名称注册 SpringBean 对象
	RegisterNameBean(name string, bean SpringBean)

	// 使用默认的名称注册 Singleton SpringBean 对象
	RegisterSingletonBean(bean SpringBean)

	// 使用指定的名称注册 Singleton SpringBean 对象
	RegisterSingletonNameBean(name string, bean SpringBean)

	// 通过 SpringBeanDefinition 注册 SpringBean 对象
	RegisterBeanDefinition(d *SpringBeanDefinition)

	// 根据 Bean 类型查找 SpringBean
	FindBeanByType(i interface{}) SpringBean

	// 根据 Bean 类型查找
	GetBeanByType(i interface{})

	// 根据 Bean 类型查找 SpringBean 数组
	FindBeansByType(i interface{})

	// 根据 Bean 类型查找 SpringBeanDefinition 数组
	FindBeanDefinitionsByType(t reflect.Type) []*SpringBeanDefinition

	// 根据 Bean 名称查找 SpringBean
	FindBeanByName(name string) SpringBean

	// 根据 Bean 名称查找 SpringBeanDefinition
	FindBeanDefinitionByName(name string) *SpringBeanDefinition

	// 获取所有的bean name
	GetAllBeanNames() []string

	// 获取属性值
	GetProperties(name string) interface{}

	// 设置属性值
	SetProperties(name string, value interface{})

	// 获取指定前缀的属性值集合
	GetPrefixProperties(prefix string) map[string]interface{}

	// 获取属性值，如果没有找到则使用指定的默认值
	GetDefaultProperties(name string, defaultValue interface{}) (interface{}, bool)

	// 自动绑定所有的 SpringBean
	AutoWireBeans() error

	// 绑定外部指定的 SpringBean
	WireBean(bean SpringBean) error
}
