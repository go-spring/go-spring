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
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/util"
)

// Properties 属性列表接口
type Properties interface {

	// Load 从文件中加载属性列表，支持 properties、yaml、toml 等文件格式。
	Load(filename string) error

	// Read 从内存中读取属性列表，支持 properties、yaml、toml 等文件格式。
	Read(b []byte, ext string) error

	// Has 返回 key 转为小写后精确匹配的属性值是否存在。
	Has(key string) bool

	// Get 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
	Get(key string) interface{}

	// Set 设置 key 转为小写后对应的属性值，key 存在则覆盖原值。
	Set(key string, value interface{})

	// Range 遍历所有的属性，属性名都为小写。
	Range(fn func(string, interface{}))

	// Bind 根据类型获取属性值，key 转为小写。
	Bind(key string, i interface{}) error

	// First 返回 keys 中第一个存在的属性值，属性名转为小写后进行精确匹配。
	First(keys ...string) interface{}

	// Default 返回 key 转为小写后精确匹配的属性值，不存在则返回 def 值。
	Default(key string, def interface{}) interface{}

	// Prefix 返回 key 转为小写后作为前缀的所有属性的集合。
	Prefix(key string) map[string]interface{}
}

// properties Properties 的默认实现。
type properties struct{ m map[string]interface{} }

// New 创建一个空的属性列表。
func New() *properties {
	return &properties{m: make(map[string]interface{})}
}

// Map 返回从 map 集合创建的属性列表。
func Map(m map[string]interface{}) *properties {
	return &properties{m: m}
}

// Load 返回从文件中读取的属性列表，支持 properties、yaml、toml 等格式。
func Load(filename string) (*properties, error) {
	p := New()
	if err := p.Load(filename); err != nil {
		return nil, err
	}
	return p, nil
}

// Read 返回从内存中读取的属性列表，支持 properties、yaml、toml 等格式。
func Read(b []byte, ext string) (*properties, error) {
	p := New()
	if err := p.Read(b, ext); err != nil {
		return nil, err
	}
	return p, nil
}

// Load 从文件中加载属性列表，支持 properties、yaml、toml 等文件格式。
func (p *properties) Load(filename string) error {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, r := range readers {
		if util.ContainsString(r.FileExt(), ext) >= 0 {
			return r.ReadFile(filename, p.m)
		}
	}
	return fmt.Errorf("unsupported file type %s", ext)
}

// Read 从内存中读取属性列表，支持 properties、yaml、toml 等文件格式。
func (p *properties) Read(b []byte, ext string) error {
	for _, r := range readers {
		if util.ContainsString(r.FileExt(), ext) >= 0 {
			return r.ReadBuffer(b, p.m)
		}
	}
	return fmt.Errorf("unsupported file type %s", ext)
}

// Has 返回 key 转为小写后精确匹配的属性值是否存在。
func (p *properties) Has(key string) bool {
	_, ok := p.m[strings.ToLower(key)]
	return ok
}

// Get 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
func (p *properties) Get(key string) interface{} {
	if v, ok := p.m[strings.ToLower(key)]; ok {
		return v
	}
	return nil
}

// Set 设置 key 转为小写后对应的属性值，key 存在则覆盖原值。
func (p *properties) Set(key string, value interface{}) {
	p.m[strings.ToLower(key)] = value
}

// Range 遍历所有的属性，属性名都为小写。
func (p *properties) Range(fn func(string, interface{})) {
	for key, val := range p.m {
		fn(key, val)
	}
}

// Bind 根据类型获取属性值，key 转为小写。
func (p *properties) Bind(key string, i interface{}) error {

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("i 必须是一个指针")
	}

	t := v.Type().Elem()
	s := t.Name() // 当绑定对象是 map 或者 slice 时，取元素的类型名
	if s == "" && (t.Kind() == reflect.Map || t.Kind() == reflect.Slice) {
		s = t.Elem().Name()
	}

	return BindValue(p, v.Elem(), "${"+key+"}", BindOption{Path: s, Key: key})
}

// First 返回 keys 中第一个存在的属性值，属性名转为小写后进行精确匹配。
func (p *properties) First(keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := p.m[strings.ToLower(key)]; ok {
			return v
		}
	}
	return nil
}

// Default 返回 key 转为小写后精确匹配的属性值，不存在则返回 def 值。
func (p *properties) Default(key string, def interface{}) interface{} {
	if v, ok := p.m[strings.ToLower(key)]; ok {
		return v
	}
	return def
}

// Prefix 返回 key 转为小写后作为前缀的所有属性的集合。
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
