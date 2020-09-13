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
	"context"
)

type GoFunc func()

// SpringContext 定义了 IoC 容器接口。
//
// 它的工作过程可以分为三个大的阶段：注册 Bean 列表、加载属性配置
// 文件、自动绑定。其中自动绑定又分为两个小阶段：解析（决议）和绑定。
//
// 一条需要谨记的注册规则是: AutoWireBeans 调用后就不能再注册新
// 的 Bean 了，这样做是因为实现起来更简单而且性能更高。
type SpringContext interface {

	// 属性值列表接口
	Properties

	// Context 返回上下文接口
	Context() context.Context

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

	// Deprecated: Use "RegisterMethodBeanFn" instead.
	// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
	// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
	// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
	RegisterMethodBean(selector BeanSelector, method string, tags ...string) *BeanDefinition

	// Deprecated: Use "RegisterNameMethodBeanFn" instead.
	// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
	// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
	RegisterNameMethodBean(name string, selector BeanSelector, method string, tags ...string) *BeanDefinition

	// @Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
	// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
	RegisterMethodBeanFn(method interface{}, tags ...string) *BeanDefinition

	// @Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
	// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
	RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *BeanDefinition

	// RegisterBeanDefinition 注册 BeanDefinition 对象，如果需要 Name 请在调用之前准备好。
	RegisterBeanDefinition(bd *BeanDefinition) *BeanDefinition

	// AutoWireBeans 对所有 Bean 进行依赖注入和属性绑定
	AutoWireBeans()

	// WireBean 对外部的 Bean 进行依赖注入和属性绑定
	WireBean(i interface{})

	// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
	GetBean(i interface{}, selector ...BeanSelector) bool

	// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
	// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
	FindBean(selector BeanSelector) (*BeanDefinition, bool)

	// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
	// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
	// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
	// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
	// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
	// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
	// selectors 列表的顺序对收集结果进行排序。
	CollectBeans(i interface{}, selectors ...BeanSelector) bool

	// GetBeanDefinitions 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
	GetBeanDefinitions() []*BeanDefinition

	// Close 关闭容器上下文，用于通知 Bean 销毁等。
	// 该函数可以确保 Bean 的销毁顺序和注入顺序相反。
	Close(beforeDestroy ...func())

	// Run 根据条件判断是否立即执行一个一次性的任务
	Run(fn interface{}, tags ...string) *Runner

	// RunNow 立即执行一个一次性的任务
	RunNow(fn interface{}, tags ...string) error

	// Config 注册一个配置函数
	Config(fn interface{}, tags ...string) *Configer

	// ConfigWithName 注册一个配置函数，名称的作用是对 Config 进行排重和排顺序。
	ConfigWithName(name string, fn interface{}, tags ...string) *Configer

	// SafeGoroutine 安全地启动一个 goroutine
	SafeGoroutine(fn GoFunc)
}

// SpringContext 的前缀看起来没有必要，想换个名字看看。
type ApplicationContext = SpringContext

// DefaultApplicationContext 创建 ApplicationContext 实例
var DefaultApplicationContext = NewDefaultSpringContext
