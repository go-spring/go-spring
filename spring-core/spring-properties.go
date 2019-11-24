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
// 定义属性值列表接口
//
type Properties interface {
	// 加载属性配置文件
	LoadProperties(filename string)

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

	// 获取字符串数组属性值，属性名称不支持大小写。
	GetStringSliceProperty(name string) []string

	// 获取哈希表数组属性值，属性名称不支持大小写。
	GetMapSliceProperty(name string) []map[string]interface{}

	// 获取属性值，如果没有找到则使用指定的默认值，属性名称不支持大小写。
	GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool)

	// 设置属性值，属性名称不支持大小写。
	SetProperty(name string, value interface{})

	// 获取指定前缀的属性值集合，属性名称不支持大小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// 获取所有的属性值
	GetAllProperties() map[string]interface{}
}

//
// 类型转换器的集合
//
var typeConverters = make(map[reflect.Type]interface{})

//
// 注册类型转换器，用于属性绑定，函数原型 func(string)struct
//
func RegisterTypeConverter(fn interface{}) {

	t := reflect.TypeOf(fn)

	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 1 {
		panic("fn must be func(string)struct")
	}

	in := t.In(0)
	out := t.Out(0)

	if in.Kind() != reflect.String || out.Kind() != reflect.Struct {
		panic("fn must be func(string)struct")
	}

	typeConverters[out] = fn
}
