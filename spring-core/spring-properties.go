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
	"errors"
	"reflect"
)

// 定义属性值接口
type Properties interface {
	// LoadProperties 加载属性配置文件
	LoadProperties(filename string)

	// GetProperty 返回属性值，属性名称统一转成小写。
	GetProperty(name string) interface{}

	// GetBoolProperty 返回布尔型属性值，属性名称统一转成小写。
	GetBoolProperty(name string) bool

	// GetIntProperty 返回有符号整型属性值，属性名称统一转成小写。
	GetIntProperty(name string) int64

	// GetUintProperty 返回无符号整型属性值，属性名称统一转成小写。
	GetUintProperty(name string) uint64

	// GetFloatProperty 返回浮点型属性值，属性名称统一转成小写。
	GetFloatProperty(name string) float64

	// GetStringProperty 返回字符串型属性值，属性名称统一转成小写。
	GetStringProperty(name string) string

	// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
	GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool)

	// SetProperty 设置属性值，属性名称统一转成小写。
	SetProperty(name string, value interface{})

	// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// GetAllProperties 返回所有的属性值，属性名称统一转成小写。
	GetAllProperties() map[string]interface{}

	// BindProperty 根据类型获取属性值，属性名称统一转成小写。
	BindProperty(name string, i interface{})
}

// typeConverters 类型转换器集合
var typeConverters = make(map[reflect.Type]interface{})

// RegisterTypeConverter 注册类型转换器，用于属性绑定，函数原型 func(string)type
func RegisterTypeConverter(fn interface{}) {
	t := reflect.TypeOf(fn)

	var (
		outType reflect.Type
	)

	// 判断是否是合法的类型转换器
	validTypeConverter := func() bool {

		// 必须是函数
		if t.Kind() != reflect.Func {
			return false
		}

		// 只能有一个输入参数
		if t.NumIn() != 1 {
			return false
		}

		// 只能有一个输出参数
		if t.NumOut() != 1 {
			return false
		}

		inType := t.In(0)
		outType = t.Out(0)

		// 输入参数必须是字符串类型
		if inType.Kind() != reflect.String {
			return false
		}

		// 输出参数必须是值类型
		if ok := IsValueType(outType.Kind()); !ok {
			return false
		}

		return true
	}

	if ok := validTypeConverter(); !ok {
		ft := "func(string)type"
		panic(errors.New("fn must be " + ft))
	}

	typeConverters[outType] = fn
}
