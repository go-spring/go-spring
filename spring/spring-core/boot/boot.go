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

package boot

import (
	"context"

	"github.com/go-spring/spring-core/app"
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-core/log"
)

var gApp = app.NewApplication()

// SetBannerMode 设置 Banner 的显式模式
func SetBannerMode(mode app.BannerMode) {
	gApp.SetBannerMode(mode)
}

// ExpectSysProperties 期望从系统环境变量中获取到的属性，支持正则表达式
func ExpectSysProperties(pattern ...string) {
	gApp.ExpectSysProperties(pattern...)
}

// AfterPrepare 注册一个 gApp.prepare() 执行完成之后的扩展点
func AfterPrepare(fn app.AfterPrepareFunc) {
	gApp.AfterPrepare(fn)
}

var running = false

// RunApplication 快速启动 boot 应用
func Run(cfgLocation ...string) {
	running = true
	gApp.AddConfigLocation(cfgLocation...)
	gApp.Run()
}

// Exit 退出 boot 应用
func Exit() {
	gApp.ShutDown()
	running = false
}

//////////////// SpringContext ////////////////////////

func checkRunning() {
	if running {
		// 这条限制的原因是为了让代码更好看，例如在 AfterPrepare 中注册 Bean
		log.Warn("use SpringContext when you can capture it")
	}
}

// GetProfile 返回运行环境
func GetProfile() string {
	return gApp.GetProfile()
}

// SetProfile 设置运行环境
func SetProfile(profile string) {
	gApp.Profile(profile)
}

// Bean 注册 BeanDefinition 对象。
func Bean(bd *core.BeanDefinition) *core.BeanDefinition {
	checkRunning()
	return gApp.RegisterBean(bd)
}

// Ref 注册单例 Bean，不指定名称，重复注册会 panic。
func Ref(i interface{}) *core.BeanDefinition {
	checkRunning()
	return gApp.RegisterBean(core.Ref(i))
}

// Make 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func Make(fn interface{}, tags ...string) *core.BeanDefinition {
	checkRunning()
	return gApp.RegisterBean(core.Make(fn, tags...))
}

// Child 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func Child(selector core.BeanSelector, method string, tags ...string) *core.BeanDefinition {
	checkRunning()
	return gApp.RegisterBean(core.Child(selector, method, tags...))
}

// MethodFunc 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
func MethodFunc(method interface{}, tags ...string) *core.BeanDefinition {
	checkRunning()
	return gApp.RegisterBean(core.MethodFunc(method, tags...))
}

// WireBean 对外部的 Bean 进行依赖注入和属性绑定
func WireBean(bean interface{}) {
	gApp.WireBean(bean)
}

// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func GetBean(i interface{}, selector ...core.BeanSelector) bool {
	return gApp.GetBean(i, selector...)
}

// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
func FindBean(selector core.BeanSelector) (*core.BeanDefinition, bool) {
	return gApp.FindBean(selector)
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func CollectBeans(i interface{}, selectors ...core.BeanSelector) bool {
	return gApp.CollectBeans(i, selectors...)
}

// GetBeanDefinitions 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func GetBeanDefinitions() []*core.BeanDefinition {
	return gApp.GetBeanDefinitions()
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func BindProperty(key string, i interface{}) error {
	return gApp.BindProperty(key, i)
}

// GetProperty 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func GetProperty(key string) interface{} {
	return gApp.GetProperty(key)
}

// GetFirstProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func GetFirstProperty(keys ...string) interface{} {
	return gApp.GetFirstProperty(keys...)
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func GetDefaultProperty(key string, def interface{}) interface{} {
	return gApp.GetDefaultProperty(key, def)
}

// SetProperty 设置属性值，属性名称统一转成小写。
func SetProperty(key string, value interface{}) {
	checkRunning()
	gApp.Property(key, value)
}

// Properties 获取 Properties 对象
func Properties() core.Properties {
	return gApp.Properties()
}

// Invoke 立即执行一个一次性的任务
func Invoke(fn interface{}, tags ...string) error {
	return gApp.Invoke(fn, tags...)
}

// Config 注册一个配置函数
func Config(fn interface{}, tags ...string) *core.Configer {
	return gApp.Config(fn, tags...)
}

type GoFuncWithContext func(context.Context)

// Go 安全地启动一个 goroutine
func Go(fn GoFuncWithContext) {
	gApp.Go(func() { fn(gApp.Context()) })
}
