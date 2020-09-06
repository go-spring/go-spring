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
	"context"
	"time"

	"github.com/go-spring/boot-starter"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-web"
)

var g = &struct {
	running bool
	config  *ApplicationConfig
	ctx     SpringCore.SpringContext
}{
	config: defaultApplicationConfig(),
	ctx:    SpringCore.NewDefaultSpringContext(),
}

// SetBannerMode 设置 Banner 的显式模式
func SetBannerMode(mode BannerMode) {
	g.config.bannerMode = mode
}

// ExpectSysProperties 期望从系统环境变量中获取到的属性，支持正则表达式
func ExpectSysProperties(pattern ...string) {
	g.config.expectSysProperties = pattern
}

// AfterPrepare 注册一个 app.prepare() 执行完成之后的扩展点
func AfterPrepare(fn AfterPrepareFunc) {
	g.config.listOfAfterPrepare = append(g.config.listOfAfterPrepare, fn)
}

// AppBuilder application 的构造器
type AppBuilder struct{}

// NewApplication AppBuilder 的构造函数
func NewApplication() *AppBuilder {
	return &AppBuilder{}
}

// Run 快速启动 SpringBoot 应用
func (cfg *AppBuilder) Run(configLocation ...string) {

	defer func() { g.running = false }()
	g.running = true

	BootStarter.Run(newApplication(g.ctx, *g.config, configLocation...))
}

// RunApplication 快速启动 SpringBoot 应用
func RunApplication(configLocation ...string) {
	NewApplication().Run(configLocation...)
}

// Exit 退出 SpringBoot 应用
func Exit() {
	BootStarter.Exit()
}

//////////////// SpringContext ////////////////////////

func checkRunning() {
	if g.running {
		// 这条限制的原因是为了让代码更好看，例如在 AfterPrepare 中注册 Bean
		SpringLogger.Warn("use SpringContext when you can capture it")
	}
}

// GetProfile 返回运行环境
func GetProfile() string {
	return g.ctx.GetProfile()
}

// SetProfile 设置运行环境
func SetProfile(profile string) {
	g.ctx.SetProfile(profile)
}

// AllAccess 返回是否允许访问私有字段
func AllAccess() bool {
	return g.ctx.AllAccess()
}

// SetAllAccess 设置是否允许访问私有字段
func SetAllAccess(allAccess bool) {
	g.ctx.SetAllAccess(allAccess)
}

// RegisterBean 注册单例 Bean，不指定名称，重复注册会 panic。
func RegisterBean(bean interface{}) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterBean(bean)
}

// RegisterNameBean 注册单例 Bean，需指定名称，重复注册会 panic。
func RegisterNameBean(name string, bean interface{}) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameBean(name, bean)
}

// RegisterBeanFn 注册单例构造函数 Bean，不指定名称，重复注册会 panic。
func RegisterBeanFn(fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterBeanFn(fn, tags...)
}

// RegisterNameBeanFn 注册单例构造函数 Bean，需指定名称，重复注册会 panic。
func RegisterNameBeanFn(name string, fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameBeanFn(name, fn, tags...)
}

// RegisterMethodBean 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func RegisterMethodBean(selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterMethodBean(selector, method, tags...)
}

// RegisterNameMethodBean 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// 必须给定方法名而不能通过遍历方法列表比较方法类型的方式获得函数名，因为不同方法的类型可能相同。
// 而且 interface 的方法类型不带 receiver 而成员方法的类型带有 receiver，两者类型也不好匹配。
func RegisterNameMethodBean(name string, selector SpringCore.BeanSelector, method string, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameMethodBean(name, selector, method, tags...)
}

// @Incubate 注册成员方法单例 Bean，不指定名称，重复注册会 panic。
// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
func RegisterMethodBeanFn(method interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterMethodBeanFn(method, tags...)
}

// @Incubate 注册成员方法单例 Bean，需指定名称，重复注册会 panic。
// method 形如 ServerInterface.Consumer (接口) 或 (*Server).Consumer (类型)。
func RegisterNameMethodBeanFn(name string, method interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameMethodBeanFn(name, method, tags...)
}

// WireBean 对外部的 Bean 进行依赖注入和属性绑定
func WireBean(bean interface{}) {
	g.ctx.WireBean(bean)
}

// GetBean 获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 FindBean 的区别是它在调用后能够保证返回的 Bean 已经完成了注入和绑定过程。
func GetBean(i interface{}, selector ...SpringCore.BeanSelector) bool {
	return g.ctx.GetBean(i, selector...)
}

// FindBean 查询单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
// 它和 GetBean 的区别是它在调用后不能保证返回的 Bean 已经完成了注入和绑定过程。
func FindBean(selector SpringCore.BeanSelector) (*SpringCore.BeanDefinition, bool) {
	return g.ctx.FindBean(selector)
}

// CollectBeans 收集数组或指针定义的所有符合条件的 Bean，收集到返回 true，否则返
// 回 false。该函数有两种模式:自动模式和指定模式。自动模式是指 selectors 参数为空，
// 这时候不仅会收集符合条件的单例 Bean，还会收集符合条件的数组 Bean (是指数组的元素
// 符合条件，然后把数组元素拆开一个个放到收集结果里面)。指定模式是指 selectors 参数
// 不为空，这时候只会收集单例 Bean，而且要求这些单例 Bean 不仅需要满足收集条件，而且
// 必须满足 selector 条件。另外，自动模式下不对收集结果进行排序，指定模式下根据
// selectors 列表的顺序对收集结果进行排序。
func CollectBeans(i interface{}, selectors ...SpringCore.BeanSelector) bool {
	return g.ctx.CollectBeans(i, selectors...)
}

// GetBeanDefinitions 获取所有 Bean 的定义，不能保证解析和注入，请谨慎使用该函数!
func GetBeanDefinitions() []*SpringCore.BeanDefinition {
	return g.ctx.GetBeanDefinitions()
}

// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func GetProperty(keys ...string) interface{} {
	return g.ctx.GetProperty(keys...)
}

// GetBoolProperty 返回 keys 中第一个存在的布尔型属性值，属性名称统一转成小写。
func GetBoolProperty(keys ...string) bool {
	return g.ctx.GetBoolProperty(keys...)
}

// GetIntProperty 返回 keys 中第一个存在的有符号整型属性值，属性名称统一转成小写。
func GetIntProperty(keys ...string) int64 {
	return g.ctx.GetIntProperty(keys...)
}

// GetUintProperty 返回 keys 中第一个存在的无符号整型属性值，属性名称统一转成小写。
func GetUintProperty(keys ...string) uint64 {
	return g.ctx.GetUintProperty(keys...)
}

// GetFloatProperty 返回 keys 中第一个存在的浮点型属性值，属性名称统一转成小写。
func GetFloatProperty(keys ...string) float64 {
	return g.ctx.GetFloatProperty(keys...)
}

// GetStringProperty 返回 keys 中第一个存在的字符串型属性值，属性名称统一转成小写。
func GetStringProperty(keys ...string) string {
	return g.ctx.GetStringProperty(keys...)
}

// GetDurationProperty 返回 keys 中第一个存在的 Duration 类型属性值，属性名称统一转成小写。
func GetDurationProperty(keys ...string) time.Duration {
	return g.ctx.GetDurationProperty(keys...)
}

// GetTimeProperty 返回 keys 中第一个存在的 Time 类型的属性值，属性名称统一转成小写。
func GetTimeProperty(keys ...string) time.Time {
	return g.ctx.GetTimeProperty(keys...)
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func GetDefaultProperty(key string, def interface{}) (interface{}, bool) {
	return g.ctx.GetDefaultProperty(key, def)
}

// SetProperty 设置属性值，属性名称统一转成小写。
func SetProperty(key string, value interface{}) {
	checkRunning()
	g.ctx.SetProperty(key, value)
}

// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
func GetPrefixProperties(prefix string) map[string]interface{} {
	return g.ctx.GetPrefixProperties(prefix)
}

// GetProperties 返回所有的属性值，属性名称统一转成小写。
func GetProperties() map[string]interface{} {
	return g.ctx.GetProperties()
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func BindProperty(key string, i interface{}) {
	g.ctx.BindProperty(key, i)
}

// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
func BindPropertyIf(key string, i interface{}, allAccess bool) {
	g.ctx.BindPropertyIf(key, i, allAccess)
}

// Run 根据条件判断是否立即执行一个一次性的任务
func Run(fn interface{}, tags ...string) *SpringCore.Runner {
	return g.ctx.Run(fn, tags...)
}

// RunNow 立即执行一个一次性的任务
func RunNow(fn interface{}, tags ...string) error {
	return g.ctx.RunNow(fn, tags...)
}

// Config 注册一个配置函数
func Config(fn interface{}, tags ...string) *SpringCore.Configer {
	return g.ctx.Config(fn, tags...)
}

// ConfigWithName 注册一个配置函数，名称的作用是对 Config 进行排重和排顺序。
func ConfigWithName(name string, fn interface{}, tags ...string) *SpringCore.Configer {
	return g.ctx.ConfigWithName(name, fn, tags...)
}

type GoFuncWithContext func(context.Context)

// Go 安全地启动一个 goroutine
func Go(fn GoFuncWithContext) {
	g.ctx.SafeGoroutine(func() { fn(g.ctx.Context()) })
}

/////////////////// Web Filter Register /////////////////////

// RegisterFilter 注册 Web Filter 对象 Bean，不指定名称，重复注册会 panic。
func RegisterFilter(bean interface{}) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterBean(bean).Export((*SpringWeb.Filter)(nil))
}

// RegisterNameFilter 注册 Web Filter 对象 Bean，需指定名称，重复注册会 panic。
func RegisterNameFilter(name string, bean interface{}) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameBean(name, bean).Export((*SpringWeb.Filter)(nil))
}

// RegisterFilterFn 注册 Web Filter 构造函数 Bean，不指定名称，重复注册会 panic。
func RegisterFilterFn(fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterBeanFn(fn, tags...).Export((*SpringWeb.Filter)(nil))
}

// RegisterNameFilterFn 注册 Web Filter 构造函数 Bean，需指定名称，重复注册会 panic。
func RegisterNameFilterFn(name string, fn interface{}, tags ...string) *SpringCore.BeanDefinition {
	checkRunning()
	return g.ctx.RegisterNameBeanFn(name, fn, tags...).Export((*SpringWeb.Filter)(nil))
}
