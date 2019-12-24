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

// 实现了一个完善的 IoC 容器。
package SpringCore

// 定义 IoC 容器接口，Bean 的注册规则：
//   1. 只能注册单例 Bean。
//   2. AutoWireBeans 开始后不允许注册新的 Bean（性能考虑）。
//   3. 原型 Bean 只能通过 BeanFactory 的形式使用，参见测试用例。
type SpringContext interface {
	// SpringContext 的工作过程分为三个阶段：
	// 1) 加载 Properties 文件，
	// 2) 注册 Bean 列表，
	// 3) 自动绑定，又分为两个小阶段：
	//    3.1) 判断 Bean 的注册条件，
	//    3.2) 绑定 Bean 的非直接依赖项，
	//    3.3) 绑定 Bean。

	// 属性值列表接口
	Properties

	// 获取运行环境
	GetProfile() string

	// 设置运行环境
	SetProfile(profile string)

	// 注册单例 Bean，不指定名称，重复注册会 panic。
	RegisterBean(bean interface{}) *BeanDefinition

	// 注册单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameBean(name string, bean interface{}) *BeanDefinition

	// 通过构造函数注册单例 Bean，不指定名称，重复注册会 panic。
	RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition

	// 通过构造函数注册单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition

	// 通过成员方法注册单例 Bean，不指定名称，重复注册会 panic。
	RegisterMethodBean(parent *BeanDefinition, method string, tags ...string) *BeanDefinition

	// 通过成员方法注册单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameMethodBean(name string, parent *BeanDefinition, method string, tags ...string) *BeanDefinition

	// 执行自动绑定过程
	AutoWireBeans()

	// 绑定外部指定的 Bean
	WireBean(bean interface{})

	// 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 什么情况下会多于 1 个？假设 StructA 实现了 InterfaceT，而且用户在注
	// 册时使用了 StructA 的指针注册多个 Bean，如果在获取时使用 InterfaceT,
	// 则必然出现多于 1 个的情况。
	GetBean(i interface{}) bool

	// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 什么情况下会多于 1 个？假设 StructA 和 StructB 都实现了 InterfaceT，
	// 而且用户在注册时使用了相同的名称分别注册了 StructA 和 StructB 的 Bean，
	// 这时候如果使用 InterfaceT 去获取，就会出现多于 1 个的情况。
	GetBeanByName(beanId string, i interface{}) bool

	// 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
	// 什么情况下可以使用此功能？假设 HandlerA 和 HandlerB 都实现了 HandlerT 接口，
	// 而且用户分别注册了一个 HandlerA 和 HandlerB 对象，如果用户想要同时获取 HandlerA
	// 和 HandlerB 对象，那么他可以通过 []HandlerT 即数组的方式获取到所有 Bean。
	CollectBeans(i interface{}) bool

	// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 该方法不能保证 Bean 已经执行依赖注入和属性绑定，仅供查询 Bean 是否存在。
	FindBeanByName(beanId string) (*BeanDefinition, bool)

	// 获取所有 Bean 的定义，一般仅供调试使用。
	GetAllBeanDefinitions() []*BeanDefinition
}
