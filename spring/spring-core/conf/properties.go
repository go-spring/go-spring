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

package conf

import (
	"errors"
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/log"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Converter 类型转换器，函数原型 func(string)(type,error)
type Converter interface{}

// Properties 定义属性值接口
type Properties interface {

	// Load 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
	Load(filename string) error

	// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
	Read(reader io.Reader, configType string) error

	// Convert 添加类型转换器
	Convert(fn Converter) error

	// Converters 返回类型转换器集合
	Converters() map[reflect.Type]Converter

	// Has 查询属性值是否存在，属性名称统一转成小写。
	Has(key string) bool

	// Bind 根据类型获取属性值，属性名称统一转成小写。
	Bind(key string, i interface{}) error

	// Get 返回属性值，不能存在返回 nil，属性名称统一转成小写。
	Get(key string) interface{}

	// GetFirst 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
	GetFirst(keys ...string) interface{}

	// GetDefault 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
	GetDefault(key string, def interface{}) interface{}

	// Set 设置属性值，属性名称统一转成小写。
	Set(key string, value interface{})

	// Keys 返回所有键，属性名称统一转成小写。
	Keys() []string

	// Range 遍历所有的属性值，属性名称统一转成小写。
	Range(fn func(string, interface{}))

	// Fill 填充所有的属性值，属性名称统一转成小写。
	Fill(properties map[string]interface{})

	// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
	Prefix(key string) map[string]interface{}

	// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
	Group(key string) map[string]map[string]interface{}
}

// properties Properties 的默认实现
type properties struct {
	properties map[string]interface{}
	converters map[reflect.Type]Converter
}

// New properties 的构造函数
func New() *properties {

	p := &properties{
		properties: make(map[string]interface{}),
		converters: make(map[reflect.Type]Converter),
	}

	// 注册时长转换函数 string -> time.Duration converter
	// time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"。
	_ = p.Convert(func(s string) (time.Duration, error) { return cast.ToDurationE(s) })

	// 注册日期转换函数 string -> time.Time converter
	// 支持非常多的日期格式，参见 cast.StringToDate。
	_ = p.Convert(func(s string) (time.Time, error) { return cast.ToTimeE(s) })

	return p
}

// Load 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (p *properties) Load(filename string) error {
	log.Debug("load properties from file: ", filename)

	return p.read(func(v *viper.Viper) error {
		v.SetConfigFile(filename)
		return v.ReadInConfig()
	})
}

// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (p *properties) Read(reader io.Reader, configType string) error {
	log.Debug("load properties from reader type: ", configType)

	return p.read(func(v *viper.Viper) error {
		v.SetConfigType(configType)
		return v.ReadConfig(reader)
	})
}

func (p *properties) read(reader func(*viper.Viper) error) error {

	v := viper.New()
	if err := reader(v); err != nil {
		return err
	}

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.Set(key, val)
		log.Tracef("%s=%v", key, val)
	}
	return nil
}

// validConverter 返回是否是合法的类型转换器。
func validConverter(t reflect.Type) bool {

	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 2 {
		return false
	}

	if t.In(0).Kind() != reflect.String {
		return false
	}

	return bean.IsValueType(t.Out(0).Kind()) && t.Out(1) == errorType
}

// Convert 添加类型转换器
func (p *properties) Convert(fn Converter) error {
	if t := reflect.TypeOf(fn); validConverter(t) {
		p.converters[t.Out(0)] = fn
		return nil
	}
	return errors.New("fn must be func(string)(type,error)")
}

// Converters 返回类型转换器集合
func (p *properties) Converters() map[reflect.Type]Converter {
	return p.converters
}

// Has 查询属性值是否存在，属性名称统一转成小写。
func (p *properties) Has(key string) bool {
	_, ok := p.properties[strings.ToLower(key)]
	return ok
}

// Get 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func (p *properties) Get(key string) interface{} {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v
	}
	return nil
}

// GetFirst 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *properties) GetFirst(keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := p.properties[strings.ToLower(key)]; ok {
			return v
		}
	}
	return nil
}

// GetDefault 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *properties) GetDefault(key string, def interface{}) interface{} {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v
	}
	return def
}

// Set 设置属性值，属性名称统一转成小写。
func (p *properties) Set(key string, value interface{}) {
	p.properties[strings.ToLower(key)] = value
}

// Keys 返回所有键，属性名称统一转成小写。
func (p *properties) Keys() []string {
	var keys []string
	for k := range p.properties {
		keys = append(keys, k)
	}
	return keys
}

// Range 遍历所有的属性值，属性名称统一转成小写。
func (p *properties) Range(fn func(string, interface{})) {
	for key, val := range p.properties {
		fn(key, val)
	}
}

// Fill 返回所有的属性值，属性名称统一转成小写。
func (p *properties) Fill(properties map[string]interface{}) {
	for key, val := range p.properties {
		properties[key] = val
	}
}

// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *properties) Prefix(key string) map[string]interface{} {
	key = strings.ToLower(key)
	result := make(map[string]interface{})
	for k, v := range p.properties {
		if k == key || strings.HasPrefix(k, key+".") {
			result[k] = v
		}
	}
	return result
}

// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *properties) Group(key string) map[string]map[string]interface{} {
	key = strings.ToLower(key) + "."
	result := make(map[string]map[string]interface{})
	for k, v := range p.properties {
		if strings.HasPrefix(k, key) {
			ss := strings.SplitN(k[len(key):], ".", 2)
			group := ss[0]
			m, ok := result[group]
			if !ok {
				m = make(map[string]interface{})
				result[group] = m
			}
			m[k] = v
		}
	}
	return result
}

// Bind 根据类型获取属性值，属性名称统一转成小写。
func (p *properties) Bind(key string, i interface{}) error {

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("参数 v 必须是一个指针")
	}

	t := v.Type().Elem()
	s := t.Name() // 当绑定对象是 map 或者 slice 时，取元素的类型名
	if s == "" && (t.Kind() == reflect.Map || t.Kind() == reflect.Slice) {
		s = t.Elem().Name()
	}

	return bindValue(p, v.Elem(), key, nil, BindOption{FieldName: s, FullName: key})
}
