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
	"github.com/go-spring/go-spring/boot-starter"
	"github.com/go-spring/go-spring/spring-core"
)

//
// 全局的 SpringContext 变量。
//
var ctx = SpringCore.NewDefaultSpringContext()

//
// 快速启动 SpringBoot 应用。
//
func RunApplication(configLocation ...string) {

	appCtx := &DefaultApplicationContext{
		DefaultSpringContext: ctx,
	}

	app := &Application{
		AppContext:     appCtx,
		ConfigLocation: configLocation,
	}

	BootStarter.Run(app)
}

//
// 退出 SpringBoot 应用。
//
func Exit() {
	BootStarter.Exit()
}

//////////////// SpringContext ////////////////////////

//
// 注册单例 Bean，不指定名称，重复注册会 panic。
//
func RegisterBean(bean interface{}) *SpringCore.Conditional {
	return ctx.RegisterBean(bean)
}

//
// 注册单例 Bean，需指定名称，重复注册会 panic。
//
func RegisterNameBean(name string, bean interface{}) *SpringCore.Conditional {
	return ctx.RegisterNameBean(name, bean)
}

//
// 通过构造函数注册单例 Bean，不指定名称，重复注册会 panic。
//
func RegisterBeanFn(fn interface{}, tags ...SpringCore.TagList) *SpringCore.Conditional {
	return ctx.RegisterBeanFn(fn, tags...)
}

//
// 通过构造函数注册单例 Bean，需指定名称，重复注册会 panic。
//
func RegisterNameBeanFn(name string, fn interface{}, tags ...SpringCore.TagList) *SpringCore.Conditional {
	return ctx.RegisterNameBeanFn(name, fn, tags...)
}

//
// 注册单例 Bean，使用 BeanDefinition 对象，重复注册会 panic。
//
func RegisterBeanDefinition(beanDefinition *SpringCore.BeanDefinition) *SpringCore.Conditional {
	return ctx.RegisterBeanDefinition(beanDefinition)
}

//
// 根据类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func GetBean(i interface{}) bool {
	return ctx.GetBean(i)
}

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func GetBeanByName(beanId string, i interface{}) bool {
	return ctx.GetBeanByName(beanId, i)
}

//
// 收集数组或指针定义的所有符合条件的 Bean 对象，收集到返回 true，否则返回 false。
//
func CollectBeans(i interface{}) bool {
	return ctx.CollectBeans(i)
}

//
// 根据名称和类型获取单例 Bean，若多于 1 个则 panic；找到返回 true 否则返回 false。
//
func FindBeanByName(beanId string) (interface{}, bool) {
	return ctx.FindBeanByName(beanId)
}

//
// 获取所有 Bean 的定义，一般仅供调试使用。
//
func GetAllBeanDefinitions() []*SpringCore.BeanDefinition {
	return ctx.GetAllBeanDefinitions()
}

//
// 加载属性配置文件
//
func LoadProperties(filename string) {
	ctx.LoadProperties(filename)
}

//
// 获取属性值，属性名称不支持大小写。
//
func GetProperty(name string) interface{} {
	return ctx.GetProperty(name)
}

//
// 获取布尔型属性值，属性名称不支持大小写。
//
func GetBoolProperty(name string) bool {
	return ctx.GetBoolProperty(name)
}

//
// 获取有符号整型属性值，属性名称不支持大小写。
//
func GetIntProperty(name string) int64 {
	return ctx.GetIntProperty(name)
}

//
// 获取无符号整型属性值，属性名称不支持大小写。
//
func GetUintProperty(name string) uint64 {
	return ctx.GetUintProperty(name)
}

//
// 获取浮点型属性值，属性名称不支持大小写。
//
func GetFloatProperty(name string) float64 {
	return ctx.GetFloatProperty(name)
}

//
// 获取字符串型属性值，属性名称不支持大小写。
//
func GetStringProperty(name string) string {
	return ctx.GetStringProperty(name)
}

//
// 获取属性值，如果没有找到则使用指定的默认值，属性名称不支持大小写。
//
func GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	return ctx.GetDefaultProperty(name, defaultValue)
}

//
// 设置属性值，属性名称不支持大小写。
//
func SetProperty(name string, value interface{}) {
	ctx.SetProperty(name, value)
}

//
// 获取指定前缀的属性值集合，属性名称不支持大小写。
//
func GetPrefixProperties(prefix string) map[string]interface{} {
	return ctx.GetPrefixProperties(prefix)
}

//
// 获取所有的属性值
//
func GetAllProperties() map[string]interface{} {
	return ctx.GetAllProperties()
}

//
// 自动绑定所有的 Bean
//
func AutoWireBeans() {
	ctx.AutoWireBeans()
}

//
// 绑定外部指定的 Bean
//
func WireBean(bean interface{}) {
	ctx.WireBean(bean)
}
