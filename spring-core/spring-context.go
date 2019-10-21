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
type BeanInitialization interface {
	InitBean(ctx SpringContext)
}

//
// SpringBean 初始化状态值
//
const (
	Uninitialized = iota // 还未初始化
	Initializing         // 正在初始化
	Initialized          // 完成初始化
)

//
// 定义 BeanDefinition 类型
//
type BeanDefinition struct {
	Bean  SpringBean    // Bean 对象
	Name  string        // Bean 名称，可能为空
	Init  int           // Bean 初始化状态
	Type  reflect.Type  // Bean 反射得到的类型
	Value reflect.Value // Bean 反射得到的值
}

//
// 定义 SpringContext 接口
//
type SpringContext interface {
	// Bean 的注册规则:
	// 1. 单例 Bean 只能注册指针和数组。
	// 2. 执行完 AutoWireBeans 后不能再注册 Bean（性能考虑）。
	// 3. 原型 Bean 只能通过 BeanFactory 的形式使用，参见测试用例。

	// 注册单例 Bean，不指定名称，重复注册会 panic。
	RegisterBean(bean SpringBean)

	// 注册单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameBean(name string, bean SpringBean)

	// 注册单例 Bean，使用 BeanDefinition 对象，重复注册会 panic。
	RegisterBeanDefinition(beanDefinition *BeanDefinition)

	// 根据类型获取单例 Bean，多于 1 个会 panic，找不到也会 panic。
	// 什么情况下会多于 1 个？假设 StructA 实现了 InterfaceT，而且用户在注
	// 册时使用了 StructA 的指针注册多个 Bean，如果在获取时使用 InterfaceT,
	// 则必然出现多于 1 个的情况。
	GetBean(i interface{})

	// 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	FindBean(i interface{}) bool

	// 根据名称和类型获取单例 Bean，多于 1 个会 panic，找不到也会 panic。
	// 什么情况下会多于 1 个？假设 StructA 和 StructB 都实现了 InterfaceT，
	// 而且用户在注册时使用了相同的名称分别注册了 StructA 和 StructB 的 Bean，
	// 这时候如果使用 InterfaceT 去获取，就会出现多于 1 个的情况。
	GetBeanByName(name string, i interface{})

	// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	FindBeanByName(name string, i interface{}) bool

	// 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
	// 什么情况下可以使用此功能？假设 HandlerA 和 HandlerB 都实现了 HandlerT 接口，
	// 而且用户分别注册了一个 HandlerA 和 HandlerB 对象，如果用户想要同时获取 HandlerA
	// 和 HandlerB 对象，那么他可以通过 []HandlerT 即数组的方式获取到所有 Bean。
	CollectBeans(i interface{}) bool

	// 收集数组或指针定义的所有符合条件的 Bean 对象，收集不到会 panic。
	MustCollectBeans(i interface{})

	// 获取所有 Bean 的定义，一般仅供调试使用。
	GetAllBeansDefinition() []*BeanDefinition

	// 获取属性值，属性名称不支持大小写。
	GetProperty(name string) interface{}

	// 获取布尔型属性值，属性名称不支持大小写。
	GetBoolProperty(name string) bool

	// 获取有符号整型属性值，属性名称不支持大小写。
	GetIntProperty(name string) int64

	// 获取无符号整型属性值，属性名称不支持大小写。
	GetUintProperty(name string) uint64

	// 获取浮点型属性值，属性名称不支持大小写。
	GetFloatProperty(name string) float64

	// 获取字符串型属性值，属性名称不支持大小写。
	GetStringProperty(name string) string

	// 获取属性值，如果没有找到则使用指定的默认值，属性名称不支持大小写。
	GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool)

	// 设置属性值，属性名称不支持大小写。
	SetProperty(name string, value interface{})

	// 获取指定前缀的属性值集合，属性名称不支持大小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// 自动绑定所有的 SpringBean
	AutoWireBeans()

	// 绑定外部指定的 SpringBean
	WireBean(bean SpringBean) error
}
