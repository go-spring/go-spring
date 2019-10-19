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

	// 注册单例 Bean，不指定名称
	RegisterBean(bean SpringBean)

	// 注册单例 Bean，需指定名称
	RegisterNameBean(name string, bean SpringBean)

	// 注册单例 Bean，使用 BeanDefinition 对象
	RegisterBeanDefinition(beanDefinition *BeanDefinition)

	// 根据类型获取单例 Bean，若多于 1 个则 panic，什么情况下会多于 1 个？
	// 假设 StructA 实现了 InterfaceT，而且用户在注册时使用了 StructA 的
	// 指针注册多个 Bean，如果在获取时使用 InterfaceT，则必然出现多于 1 个
	// 的情况。
	GetBean(i interface{})

	// 根据名称和类型获取单例 Bean，若多于 1 个则 panic，什么情况下会多于 1 个？
	// 假设 StructA 和 StructB 都实现了 InterfaceT，而且用户在注册时使用了相
	// 同的名称分别注册了 StructA 和 StructB 的 Bean，这时候如果使用
	// InterfaceT 去获取，就会出现多于 1 个的情况。
	GetBeanByName(name string, i interface{})

	// 收集数组或指针定义的所有符合条件的 Bean 对象。什么情况下可以使用此功能？
	// 假设 HandlerA 和 HandlerB 都实现了 HandlerT 接口，而且用户分别注册了
	// 一个 HandlerA 和 HandlerB 对象，如果用户想要同时获取 HandlerA 和
	// HandlerB 对象，那么他可以通过 []HandlerT 即数组的方式获取到所有 Bean。
	CollectBeans(i interface{})

	// 注册原型 Bean，使用反射创建新的 Bean
	// TODO RegisterPrototypeBean(bean SpringBean)

	// 注册原型 Bean，使用工厂创建新的 Bean
	// TODO RegisterPrototypeBeanFactory(factory func() SpringBean)

	// 获取原型 Bean 并自动完成绑定
	// TODO GetPrototypeBean(i interface{})

	// 获取所有 Bean 的定义，一般仅供调试使用。
	GetAllBeansDefinition() []*BeanDefinition

	// 获取属性值，属性名称不支持大小写。
	GetProperties(name string) interface{}

	// TODO GetIntProperties() 等。

	// 设置属性值，属性名称不支持大小写。
	SetProperties(name string, value interface{})

	// 获取指定前缀的属性值集合，属性名称不支持大小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// 获取属性值，如果没有找到则使用指定的默认值，属性名称不支持大小写。
	GetDefaultProperties(name string, defaultValue interface{}) (interface{}, bool)

	// 自动绑定所有的 SpringBean
	AutoWireBeans()

	// 绑定外部指定的 SpringBean
	WireBean(bean SpringBean) error
}
