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
	"time"

	"github.com/go-spring/go-spring/boot-starter"
	"github.com/go-spring/go-spring/spring-core"
)

// ctx 全局的 SpringContext 变量
var ctx = SpringCore.NewDefaultSpringContext()

// expectSysProperties 期望从系统环境变量中获取到的属性
var expectSysProperties = []string{`.*`}

// ExpectSysProperties 期望从系统环境变量中获取到的属性，支持正则表达式
func ExpectSysProperties(pattern ...string) {
	expectSysProperties = pattern
}

// RunApplication 快速启动 SpringBoot 应用
func RunApplication(configLocation ...string) {

	app := newApplication(&defaultApplicationContext{
		SpringContext: ctx,
	}, configLocation...)

	BootStarter.Run(app)
}

// Exit 退出 SpringBoot 应用
func Exit() {
	BootStarter.Exit()
}

//////////////// SpringContext ////////////////////////

// GetProfile 返回运行环境
func GetProfile() string {
	return ctx.GetProfile()
}

// SetProfile 设置运行环境
func SetProfile(profile string) {
	ctx.SetProfile(profile)
}

// AllAccess 返回是否允许访问私有字段
func AllAccess() bool {
	return ctx.AllAccess()
}

// SetAllAccess 设置是否允许访问私有字段
func SetAllAccess(allAccess bool) {
	ctx.SetAllAccess(allAccess)
}

// SetEventNotify 设置 Context 事件通知函数
func SetEventNotify(notify func(event SpringCore.ContextEvent)) {
	ctx.SetEventNotify(notify)
}

// RegisterBean 注册单例 Bean，不指定名称，重复注册会 panic。
func RegisterBean(bean interface{}) *SpringCore.BeanDefinition {
	return ctx.RegisterBean(bean)
}

// RegisterNameBean 注册单例 Bean，需指定名称，重复注册会 panic。
func RegisterNameBean(name string, bean interface{}) *SpringCore.BeanDefinition {
	return ctx.RegisterNameBean(name, bean)
}

// RegisterBeanFn 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func RegisterBeanFn(fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterBeanFn(fn, tags...)
}

// RegisterNameBeanFn 注册单例构造函数 Bean，需指定名称，重复注册会 panic。
func RegisterNameBeanFn(name string, fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterNameBeanFn(name, fn, tags...)
}

// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
func RegisterMethodBean(selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterMethodBean(selector, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// selector 可以是 *BeanDefinition，可以是 BeanId，还可以是 (Type)(nil) 变量。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型不好匹配。
func RegisterNameMethodBean(name string, selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterNameMethodBean(name, selector, method, tags...)
}

// @Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
func RegisterMethodBeanFn(method interface{}, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterMethodBeanFn(method, tags...)
}

// @Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
func RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *SpringCore.BeanDefinition {
	return ctx.RegisterNameMethodBeanFn(name, method, tags...)
}

// WireBean 绑定外部的 Bean 源
func WireBean(bean interface{}) {
	ctx.WireBean(bean)
}

// GetBean 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func GetBean(i interface{}) bool {
	return ctx.GetBean(i)
}

// GetBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func GetBeanByName(beanId string, i interface{}) bool {
	return ctx.GetBeanByName(beanId, i)
}

// FindBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// selector 可以是 BeanId，还可以是 (Type)(nil) 变量，Type 为接口类型时带指针。
func FindBean(selector SpringCore.BeanSelector) (*SpringCore.BeanDefinition, bool) {
	return ctx.FindBean(selector)
}

// FindBeanByName 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
func FindBeanByName(beanId string) (*SpringCore.BeanDefinition, bool) {
	return ctx.FindBeanByName(beanId)
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
func CollectBeans(i interface{}) bool {
	return ctx.CollectBeans(i)
}

// GetBeanDefinitions 获取所有 Bean 的定义，一般仅供调试使用。
func GetBeanDefinitions() []*SpringCore.BeanDefinition {
	return ctx.GetBeanDefinitions()
}

// GetProperty 返回属性值，属性名称统一转成小写。
func GetProperty(keys ...string) interface{} {
	return ctx.GetProperty(keys...)
}

// GetBoolProperty 返回布尔型属性值，属性名称统一转成小写。
func GetBoolProperty(keys ...string) bool {
	return ctx.GetBoolProperty(keys...)
}

// GetIntProperty 返回有符号整型属性值，属性名称统一转成小写。
func GetIntProperty(keys ...string) int64 {
	return ctx.GetIntProperty(keys...)
}

// GetUintProperty 返回无符号整型属性值，属性名称统一转成小写。
func GetUintProperty(keys ...string) uint64 {
	return ctx.GetUintProperty(keys...)
}

// GetFloatProperty 返回浮点型属性值，属性名称统一转成小写。
func GetFloatProperty(keys ...string) float64 {
	return ctx.GetFloatProperty(keys...)
}

// GetStringProperty 返回字符串型属性值，属性名称统一转成小写。
func GetStringProperty(keys ...string) string {
	return ctx.GetStringProperty(keys...)
}

// GetDurationProperty 返回 Duration 类型属性值，属性名称统一转成小写。
func GetDurationProperty(keys ...string) time.Duration {
	return ctx.GetDurationProperty(keys...)
}

// GetTimeProperty 返回 Time 类型的属性值，属性名称统一转成小写。
func GetTimeProperty(keys ...string) time.Time {
	return ctx.GetTimeProperty(keys...)
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func GetDefaultProperty(key string, def interface{}) (interface{}, bool) {
	return ctx.GetDefaultProperty(key, def)
}

// SetProperty 设置属性值，属性名称统一转成小写。
func SetProperty(key string, value interface{}) {
	ctx.SetProperty(key, value)
}

// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
func GetPrefixProperties(prefix string) map[string]interface{} {
	return ctx.GetPrefixProperties(prefix)
}

// GetProperties 返回所有的属性值，属性名称统一转成小写。
func GetProperties() map[string]interface{} {
	return ctx.GetProperties()
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func BindProperty(key string, i interface{}) {
	ctx.BindProperty(key, i)
}

// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
func BindPropertyIf(key string, i interface{}, allAccess bool) {
	ctx.BindPropertyIf(key, i, allAccess)
}

// Run 立即执行一个一次性的任务
func Run(fn interface{}, tags ...string) *SpringCore.Runner {
	return ctx.Run(fn, tags...)
}

// Config 注册一个配置函数
func Config(fn interface{}, tags ...string) *SpringCore.Configer {
	return ctx.Config(fn, tags...)
}

// ConfigWithName 注册一个配置函数，name 的作用：区分，排重，排顺序。
func ConfigWithName(name string, fn interface{}, tags ...string) *SpringCore.Configer {
	return ctx.ConfigWithName(name, fn, tags...)
}
