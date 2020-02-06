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

import (
	"context"
)

// ContextEvent 定义 SpringContext 事件类型
type ContextEvent int

const (
	ContextEvent_ResolveStart  = ContextEvent(0) // 开始解析 Bean 的过程
	ContextEvent_ResolveEnd    = ContextEvent(1) // 结束解析 Bean 的过程
	ContextEvent_AutoWireStart = ContextEvent(2) // 开始注入 Bean 的过程
	ContextEvent_AutoWireEnd   = ContextEvent(3) // 结束注入 Bean 的过程
	ContextEvent_CloseStart    = ContextEvent(4) // 开始关闭 Context 的过程
	ContextEvent_CloseEnd      = ContextEvent(5) // 结束关闭 Context 的过程
)

// SpringContext 定义 IoC 容器接口，Bean 的注册规则：
//   1. AutoWireBeans 开始后不允许注册新的 Bean（性能考虑）
type SpringContext interface {
	// SpringContext 的工作过程分为三个阶段：
	// 1) 加载 Properties 文件，
	// 2) 注册 Bean 列表，
	// 3) 自动绑定，又分为两个小阶段：
	//    3.1) 解析 Bean，
	//    3.2) 绑定 Bean。

	// 属性值列表接口
	Properties

	// 上下文接口
	context.Context

	// GetProfile 返回运行环境
	GetProfile() string

	// SetProfile 设置运行环境
	SetProfile(profile string)

	// AllAccess 返回是否允许访问私有字段
	AllAccess() bool

	// SetAllAccess 设置是否允许访问私有字段
	SetAllAccess(allAccess bool)

	// SetEventNotify 设置 Context 事件通知函数
	SetEventNotify(notify func(event ContextEvent))

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
	RegisterMethodBean(selector interface{}, method string, tags ...string) *BeanDefinition

	// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
	RegisterNameMethodBean(name string, selector interface{}, method string, tags ...string) *BeanDefinition

	// RegisterMethodBeanFn 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
	RegisterMethodBeanFn(method interface{}, tags ...string) *BeanDefinition

	// RegisterNameMethodBeanFn 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *BeanDefinition

	// AutoWireBeans 完成自动绑定
	AutoWireBeans()

	// WireBean 绑定外部的 Bean 源
	WireBean(bean interface{})

	// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 什么情况下会多于 1 个？假设 StructA 实现了 InterfaceT，而且用户在注册时使用了
	// StructA 的指针注册多个 Bean，如果在获取时使用 InterfaceT,则必然出现多于 1 个的情况。
	GetBean(i interface{}) bool

	// GetBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 什么情况下会多于 1 个？假设 StructA 和 StructB 都实现了 InterfaceT，而且用户在注册时使用了相
	// 同的名称分别注册了 StructA 和 StructB 的 Bean，这时候如果使用 InterfaceT 去获取，就会出现多于 1 个的情况。
	GetBeanByName(beanId string, i interface{}) bool

	// FindBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	FindBeanByName(beanId string) (*BeanDefinition, bool)

	// FindBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// selector 可以是 BeanId，还可以是 (Type)(nil) 变量，Type 为接口类型时带指针。
	FindBean(selector interface{}) (*BeanDefinition, bool)

	// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
	// 什么情况下可以使用此功能？假设 HandlerA 和 HandlerB 都实现了 HandlerT 接口，而且用户分别注册
	// 了一个 HandlerA 和 HandlerB 对象，如果用户想要同时获取 HandlerA 和 HandlerB 对象，那么他可
	// 以通过 []HandlerT 即数组的方式获取到所有 Bean。
	CollectBeans(i interface{}) bool

	// GetBeanDefinitions 获取所有 Bean 的定义，一般仅供调试使用。
	GetBeanDefinitions() []*BeanDefinition

	// Close 关闭容器上下文，用于通知 Bean 销毁等。
	Close()
}
