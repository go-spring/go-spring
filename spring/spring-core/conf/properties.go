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
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/util"
)

// errorType error 的反射类型
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// Properties 定义属性值接口
type Properties interface {

	// Load 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
	Load(filename string) error

	// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
	Read(b []byte, ext string) error

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
	m map[string]interface{}
}

// New properties 的构造函数
func New() *properties {
	return &properties{m: make(map[string]interface{})}
}

// Map properties 的构造函数
func Map(m map[string]interface{}) *properties {
	return &properties{m: m}
}

// Load properties 的构造函数
func Load(filename string) (*properties, error) {
	p := New()
	if err := p.Load(filename); err != nil {
		return nil, err
	}
	return p, nil
}

// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
func Read(b []byte, ext string) (*properties, error) {
	p := New()
	if err := p.Read(b, ext); err != nil {
		return nil, err
	}
	return p, nil
}

// Load 加载属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (p *properties) Load(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, r := range Readers {
		if util.ContainsString(r.FileExt(), ext) >= 0 {
			return r.ReadFile(filename, p.m)
		}
	}
	panic(errors.New("unsupported file type"))
}

// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (p *properties) Read(b []byte, ext string) error {
	for _, r := range Readers {
		if util.ContainsString(r.FileExt(), ext) >= 0 {
			return r.ReadBuffer(b, p.m)
		}
	}
	panic(errors.New("unsupported file type"))
}

// Has 查询属性值是否存在，属性名称统一转成小写。
func (p *properties) Has(key string) bool {
	_, ok := p.m[strings.ToLower(key)]
	return ok
}

// Get 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func (p *properties) Get(key string) interface{} {
	if v, ok := p.m[strings.ToLower(key)]; ok {
		return v
	}
	return nil
}

// GetFirst 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *properties) GetFirst(keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := p.m[strings.ToLower(key)]; ok {
			return v
		}
	}
	return nil
}

// GetDefault 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *properties) GetDefault(key string, def interface{}) interface{} {
	if v, ok := p.m[strings.ToLower(key)]; ok {
		return v
	}
	return def
}

// Set 设置属性值，属性名称统一转成小写。
func (p *properties) Set(key string, value interface{}) {
	p.m[strings.ToLower(key)] = value
}

// Keys 返回所有键，属性名称统一转成小写。
func (p *properties) Keys() []string {
	var keys []string
	for k := range p.m {
		keys = append(keys, k)
	}
	return keys
}

// Range 遍历所有的属性值，属性名称统一转成小写。
func (p *properties) Range(fn func(string, interface{})) {
	for key, val := range p.m {
		fn(key, val)
	}
}

// Fill 返回所有的属性值，属性名称统一转成小写。
func (p *properties) Fill(properties map[string]interface{}) {
	for key, val := range p.m {
		properties[key] = val
	}
}

// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *properties) Prefix(key string) map[string]interface{} {
	key = strings.ToLower(key)
	result := make(map[string]interface{})
	for k, v := range p.m {
		if k == key || strings.HasPrefix(k, key+".") {
			result[k] = v
		}
	}
	return result
}

// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *properties) Group(key string) map[string]map[string]interface{} {
	return Group(key, p.m)
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

	return BindValue(p, v.Elem(), "${"+key+"}", BindOption{FieldName: s, FullName: key})
}
