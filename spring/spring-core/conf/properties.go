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
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// Properties 属性列表接口。所有的 key 都是小写，匹配的时候也都转成小写然后再匹配。
//
// 一般情况下 key 都是 a.b.c 这种形式，但是这种形式只能表达 map 嵌套的结构，而没
// 有办法获取 slice 里面的数据，想要获取 slice 里面的数据需要使用 a[0].b 这种形
// 式的 key，但是这种形式的 key 并不常用，所以 Properties 只在 Get 方法中支持这
// 种形式的 key。另外，a.[0].b 和 a[0].b 是等价的，所以可以支持 a[0].[0].b 这
// 种复杂但合理的 key。
//
// Load 和 Read 方法最终都是通过 Reader 接口读取属性列表，用户可以通过 Reader
// 注册接口来自定义需要支持的文件格式。
type Properties interface {

	// Load 从文件读取属性列表。
	Load(filename string) error

	// Read 从 []byte 读取属性列表，ext 是文件扩展名，如 .toml、.yaml 等。
	Read(b []byte, ext string) error

	// Keys 返回所有属性的 key。
	Keys() []string

	// Get 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。如果返回值是 map
	// 或者 slice 类型的数据，会返回它们深拷贝后的副本，防止因为修改了返回值而对
	// Properties 的数据造成修改。另外，Get 方法支持传入多个 key，然后返回找到的
	// 第一个属性值，如果所有的 key 都没找到对应的属性值则返回 nil。
	Get(key string) interface{}

	// Set 设置 key 对应的属性值，如果 key 存在会覆盖原值。Set 方法在保存属性的时
	// 候会将 key 转为小写，如果属性值是 map 类型或者包含 map 类型的数据，那么也会
	// 将这些 key 全部转为小写。另外，Set 方法保存的是 value 深拷贝后的副本，从而
	// 保证 Properties 数据的安全。
	Set(key string, value interface{})
}

// properties Properties 的默认实现。
type properties struct {

	// m 是一个(多维)嵌套的 map[string]interface{} 结构。
	m map[string]interface{}
}

// New 返回一个空的属性列表。
func New() Properties {
	return &properties{m: make(map[string]interface{})}
}

// Map 返回从 map 集合创建的属性列表，保存的是 map 深拷贝后的值。
func Map(m map[string]interface{}) Properties {
	m = dupValue(m, true).(map[string]interface{})
	return &properties{m: m}
}

// Load 从文件加载属性列表。
func Load(filename string) (Properties, error) {
	p := New()
	if err := p.Load(filename); err != nil {
		return nil, err
	}
	return p, nil
}

// Read 从 []byte 读取属性列表，ext 是文件扩展名，如 .toml、.yaml 等。
func Read(b []byte, configType string) (Properties, error) {
	p := New()
	if err := p.Read(b, configType); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *properties) Load(filename string) error {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	ext := filepath.Ext(filename)
	return p.Read(b, strings.TrimPrefix(ext, "."))
}

func (p *properties) Read(b []byte, configType string) error {
	configType = strings.ToLower(configType)
	if r, ok := readers[configType]; ok {
		return r.Read(p, b)
	}
	return fmt.Errorf("unsupported file type %s", configType)
}

func splitPath(path string) (key string, index int) {

	if len(path) == 0 {
		return "", -1
	}

	l := len(path) - 1
	if path[l] != ']' {
		return path, -1
	}

	i := strings.IndexByte(path, '[')
	if i < 0 {
		return path, -1
	}

	n, err := strconv.Atoi(path[i+1 : l])
	if err == nil {
		return path[:i], n
	}
	return path, -1
}

func (p *properties) find(path []string) interface{} {
	i := 0
	ok := false
	v := interface{}(p.m)
	for ; i < len(path); i++ {

		k := path[i]
		if len(k) == 0 {
			continue
		}

		key, index := splitPath(k)
		if len(key) > 0 {
			var m map[string]interface{}
			m, ok = v.(map[string]interface{})
			if !ok {
				return nil
			}
			v, ok = m[key]
			if !ok {
				return nil
			}
		}

		if index >= 0 {
			var s []interface{}
			s, ok = v.([]interface{})
			if !ok || index >= len(s) {
				return nil
			}
			v = s[index]
		}
	}
	return v
}

func (p *properties) create(path []string) map[string]interface{} {
	m := p.m
	for _, k := range path {
		m2, ok := m[k]
		if !ok {
			m3 := make(map[string]interface{})
			m[k] = m3
			m = m3
			continue
		}
		m3, ok := m2.(map[string]interface{})
		if !ok {
			m3 = make(map[string]interface{})
			m[k] = m3
		}
		m = m3
	}
	return m
}

func (p *properties) Keys() []string {
	var s []string
	for k := range util.FlatMap(p.m) {
		s = append(s, k)
	}
	return s
}

func (p *properties) Get(key string) interface{} {
	if key == "" {
		return nil
	}
	key = strings.ToLower(key)
	path := strings.Split(key, ".")
	return dupValue(p.find(path), false)
}

func (p *properties) Set(key string, value interface{}) {
	key = strings.ToLower(key)
	path := strings.Split(key, ".")
	nodeMap := p.create(path[0 : len(path)-1])
	nodeMap[path[len(path)-1]] = dupValue(value, true)
}

// dupValue 深度拷贝 value 的值，如果 value 是 map 类型或者包含 map 类型，
// 当 toLower 为 true 时表示将 map 的 key 转为小写，为 false 时表示不转换。
func dupValue(value interface{}, toLower bool) interface{} {
	switch v := value.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{})
		for k, val := range v {
			key := cast.ToString(k)
			if toLower {
				key = strings.ToLower(key)
			}
			m[key] = dupValue(val, toLower)
		}
		return m
	case map[string]interface{}:
		m := make(map[string]interface{})
		for key, val := range v {
			if toLower {
				key = strings.ToLower(key)
			}
			m[key] = dupValue(val, toLower)
		}
		return m
	case []interface{}:
		var s []interface{}
		for _, val := range v {
			s = append(s, dupValue(val, toLower))
		}
		return s
	}
	return value
}

// Bind 将 key 对应的属性值绑定到某个数据类型的实例上。i 必须是一个指针，只有这
// 样才能将修改传递出去。Bind 方法使用 tag 对结构体的字段进行属性绑定，tag 的语
// 法为 value:"${a:=b}"，其中 value 是表示属性绑定 tag 的名称，${} 表示引用
// 一个属性，a 表示属性名，:=b 表示属性的默认值。这里需要注意两点：
//
// 一是结构体类型的字段上不允许设置默认值，这个规则一方面是因为找不到合理的序列化
// 方式，有人会说可以用 json，那么肯定也会有人说用 xml，众口难调，另一方面是因为
// 结构体的默认值一般会比较长，而如果 tag 太长就会影响阅读体验，因此结构体类型的
// 字段上不允许设置默认值；
//
// 二是可以省略属性名而只有默认值，即 ${:=b}，原因是某些情况下属性名可能没想好或
// 者不太重要，也有人认为这是一种对 Golang 缺少默认值语法的补充，Bug is Feature。
//
// 另外，属性绑定语法还支持嵌套的属性引用，但是只能在默认值中使用，即 ${a:=${b}}。
func Bind(p interface{ Get(key string) interface{} }, key string, i interface{}) error {

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

// Default 返回 key 的属性值，不存在时返回 def 值。
func Default(p interface{ Get(key string) interface{} }, key string, def interface{}) interface{} {
	if v := p.Get(key); v != nil {
		return v
	}
	return def
}

// validValueTag 是否为 ${key:=def} 格式的字符串。
func validValueTag(tag string) bool {
	return strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")
}

// parseValueTag 解析 ${key:=def} 字符串，返回 key 和 def 的值。
func parseValueTag(tag string) (key string, def interface{}, err error) {

	if !validValueTag(tag) {
		return "", nil, errors.New("invalid value tag")
	}

	tag = tag[2 : len(tag)-1]
	ss := strings.SplitN(tag, ":=", 2)
	if len(ss) > 1 {
		def = ss[1]
	}
	key = ss[0]
	return
}

// Resolve 解析 ${key:=def} 字符串，返回 key 对应的属性值，如果没有找到则返回
// def 值，如果 def 存在引用关系则递归解析直到获取最终的属性值。
func Resolve(p interface{ Get(key string) interface{} }, tagOrValue interface{}) (interface{}, error) {

	// 不是字符串或者没有使用配置引用语法则直接返回
	tag, ok := tagOrValue.(string)
	if !ok || !validValueTag(tag) {
		return tagOrValue, nil
	}

	key, def, _ := parseValueTag(tag)
	if val := Default(p, key, def); val != nil {
		return Resolve(p, val)
	}

	return nil, fmt.Errorf("property %q not config", key)
}
