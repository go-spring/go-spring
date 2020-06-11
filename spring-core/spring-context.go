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

// 实现了一个功能完善的 IoC 容器。
package SpringCore

import (
	"context"
)

// SpringContext 定义 IoC 容器接口。
//
// 其工作过程分为三个大阶段：注册 Bean 列表、加载 Properties
// 文件、自动绑定。自动绑定又分为两个小阶段：解析（决议）和绑定。
//
// 一条需要谨记的注册规则是 AutoWireBeans 开始后不允许注册新的
// Bean，这样做是因为实现起来更简单而且性能更高。
type SpringContext interface {
	// 上下文接口
	context.Context

	// 属性值列表接口
	Properties

	// GetProfile 返回运行环境
	GetProfile() string

	// SetProfile 设置运行环境
	SetProfile(profile string)

	// AllAccess 返回是否允许访问私有字段
	AllAccess() bool

	// SetAllAccess 设置是否允许访问私有字段
	SetAllAccess(allAccess bool)

	// RegisterBean 注册单例 Bean，不指定名称，重复注册会 panic。
	RegisterBean(bean interface{}) *BeanDefinition

	// RegisterNameBean 注册单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameBean(name string, bean interface{}) *BeanDefinition

	// RegisterBeanFn 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
	RegisterBeanFn(fn interface{}, tags ...string) *BeanDefinition

	// RegisterNameBeanFn 注册单例构造函数 Bean，需指定名称，重复注册会 panic。
	RegisterNameBeanFn(name string, fn interface{}, tags ...string) *BeanDefinition

	// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
	// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
	// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
	// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
	RegisterMethodBean(selector BeanSelector, method string, tags ...string) *BeanDefinition

	// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
	// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
	// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
	RegisterNameMethodBean(name string, selector BeanSelector, method string, tags ...string) *BeanDefinition

	// @Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
	RegisterMethodBeanFn(method interface{}, tags ...string) *BeanDefinition

	// @Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *BeanDefinition

	// AutoWireBeans 完成自动绑定
	AutoWireBeans()

	// WireBean 绑定外部的 Bean 源
	WireBean(bean interface{})

	// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 什么情况下会多于 1 个？假设 StructA 实现了 InterfaceT，而且用户在注册时使用了
	// StructA 的指针注册多个 Bean，如果在获取时使用 InterfaceT,则必然出现多于 1 个的情况。
	GetBean(i interface{}) bool

	// FindBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// selector 可以是 BeanId，还可以是 (*Type)(nil) 变量，Type 为接口类型时带指针。
	FindBean(selector BeanSelector) (*BeanDefinition, bool)

	// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
	// 什么情况下可以使用此功能？假设 HandlerA 和 HandlerB 都实现了 HandlerT 接口，而且用户分别注册
	// 了一个 HandlerA 和 HandlerB 对象，如果用户想要同时获取 HandlerA 和 HandlerB 对象，那么他可
	// 以通过 []HandlerT 即数组的方式获取到所有 Bean。
	CollectBeans(i interface{}) bool

	// SelectBean
	SelectBean(i interface{}, selector BeanSelector) bool

	// SelectBeans
	SelectBeans(i interface{}, selector ...BeanSelector) bool

	// GetBeanDefinitions 获取所有 Bean 的定义，一般仅供调试使用。
	GetBeanDefinitions() []*BeanDefinition

	// Close 关闭容器上下文，用于通知 Bean 销毁等。
	Close()

	// Run 立即执行一个一次性的任务
	Run(fn interface{}, tags ...string) *Runner

	// Config 注册一个配置函数
	Config(fn interface{}, tags ...string) *Configer

	// ConfigWithName 注册一个配置函数，name 的作用：区分，排重，排顺序。
	ConfigWithName(name string, fn interface{}, tags ...string) *Configer
}
