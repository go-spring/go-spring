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
	"io"
	"reflect"
	"time"
)

// Properties 定义属性值接口
type Properties interface {
	// LoadProperties 加载属性配置文件，
	// 支持 properties、yaml 和 toml 三种文件格式。
	LoadProperties(filename string)

	// ReadProperties 读取属性配置文件，
	// 支持 properties、yaml 和 toml 三种文件格式。
	ReadProperties(reader io.Reader, configType string)

	// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
	GetProperty(keys ...string) interface{}

	// GetBoolProperty 返回 keys 中第一个存在的布尔型属性值，属性名称统一转成小写。
	GetBoolProperty(keys ...string) bool

	// GetIntProperty 返回 keys 中第一个存在的有符号整型属性值，属性名称统一转成小写。
	GetIntProperty(keys ...string) int64

	// GetUintProperty 返回 keys 中第一个存在的无符号整型属性值，属性名称统一转成小写。
	GetUintProperty(keys ...string) uint64

	// GetFloatProperty 返回 keys 中第一个存在的浮点型属性值，属性名称统一转成小写。
	GetFloatProperty(keys ...string) float64

	// GetStringProperty 返回 keys 中第一个存在的字符串型属性值，属性名称统一转成小写。
	GetStringProperty(keys ...string) string

	// GetDurationProperty 返回 keys 中第一个存在的 Duration 类型属性值，属性名称统一转成小写。
	GetDurationProperty(keys ...string) time.Duration

	// GetTimeProperty 返回 keys 中第一个存在的 Time 类型的属性值，属性名称统一转成小写。
	GetTimeProperty(keys ...string) time.Time

	// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
	GetDefaultProperty(key string, def interface{}) (interface{}, bool)

	// SetProperty 设置属性值，属性名称统一转成小写。
	SetProperty(key string, value interface{})

	// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// GetProperties 返回所有的属性值，属性名称统一转成小写。
	GetProperties() map[string]interface{}

	// BindProperty 根据类型获取属性值，属性名称统一转成小写。
	BindProperty(key string, i interface{})

	// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
	BindPropertyIf(key string, i interface{}, allAccess bool)
}

// typeConverters 类型转换器集合
var typeConverters = make(map[reflect.Type]interface{})

// validTypeConverter 返回是否是合法的类型转换器，类型转换器要求：
// 必须是函数，且只能有一个字符串类型的输入参数和一个值类型的输出参数。
func validTypeConverter(t reflect.Type) bool {

	// 必须是函数 && 只能有一个输入参数 && 只能有一个输出参数
	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 1 {
		return false
	}

	inType := t.In(0)
	outType := t.Out(0)

	// 输入参数必须是字符串类型 && 输出参数必须是值类型
	return inType.Kind() == reflect.String && IsValueType(outType.Kind())
}

// RegisterTypeConverter 注册类型转换器，函数原型 func(string)type
func RegisterTypeConverter(fn interface{}) {
	if t := reflect.TypeOf(fn); validTypeConverter(t) {
		typeConverters[t.Out(0)] = fn
	} else {
		panic(errors.New("fn must be func(string)type"))
	}
}
