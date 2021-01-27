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
	"io"
	"reflect"
)

// TypeConverter 类型转换器，函数原型 func(string)type
type TypeConverter interface{}

// Properties 定义属性值接口
type Properties interface {

	// LoadProperties 加载属性配置文件，
	// 支持 properties、yaml 和 toml 三种文件格式。
	LoadProperties(filename string)

	// ReadProperties 读取属性配置文件，
	// 支持 properties、yaml 和 toml 三种文件格式。
	ReadProperties(reader io.Reader, configType string)

	// AddTypeConverter 添加类型转换器
	AddTypeConverter(fn TypeConverter)

	// TypeConverters 类型转换器集合
	TypeConverters() map[reflect.Type]TypeConverter

	// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
	GetProperty(keys ...string) interface{}

	// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
	GetDefaultProperty(key string, def interface{}) (interface{}, bool)

	// SetProperty 设置属性值，属性名称统一转成小写。
	SetProperty(key string, value interface{})

	// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
	GetPrefixProperties(prefix string) map[string]interface{}

	// GetGroupedProperties 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
	GetGroupedProperties(prefix string) map[string]map[string]interface{}

	// GetProperties 返回所有的属性值，属性名称统一转成小写。
	GetProperties() map[string]interface{}

	// BindProperty 根据类型获取属性值，属性名称统一转成小写。
	BindProperty(key string, i interface{})
}
