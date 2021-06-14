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

// Package conf 提供了读取属性列表的一般方法，并且通过扩展支持各种格式的属性列表文件。
package conf

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/contain"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// RootKey 可以用它来获取完整的属性表。
const RootKey = "$"

// SpringProfile 可以用它来设置 spring 的运行环境。
const SpringProfile = "spring.profile"

type getArg struct {
	def interface{} // 默认值
}

type GetOption func(arg *getArg)

// Def 设置默认值。
func Def(v interface{}) GetOption {
	return func(arg *getArg) {
		arg.def = v
	}
}

type bindArg struct {
	tag string
}

type BindOption func(arg *bindArg)

// Key 设置绑定的 key 。
func Key(key string) BindOption {
	return func(arg *bindArg) {
		arg.tag = "${" + key + "}"
	}
}

// Tag 设置绑定的 tag 。
func Tag(tag string) BindOption {
	return func(arg *bindArg) {
		arg.tag = tag
	}
}

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
	Keys() []string

	// Has key 对应的属性值是否存在。
	Has(key string) bool

	// Get 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。如果返回值是 map
	// 或者 slice 类型的数据，会返回它们深拷贝后的副本，防止因为修改了返回值而对
	// Properties 的数据造成修改。另外，Get 方法支持传入多个 key，然后返回找到的
	// 第一个属性值，如果所有的 key 都没找到对应的属性值则返回 nil。
	Get(key string, opts ...GetOption) string

	// Set 设置 key 对应的属性值，如果 key 存在会覆盖原值。Set 方法在保存属性的时
	// 候会将 key 转为小写，如果属性值是 map 类型或者包含 map 类型的数据，那么也会
	// 将这些 key 全部转为小写。另外，Set 方法保存的是 value 深拷贝后的副本，从而
	// 保证 Properties 数据的安全。
	Set(key string, val interface{})
}

type properties struct{ m map[string]string }

// New 返回一个空的属性列表。
func New() Properties {
	return &properties{m: make(map[string]string)}
}

// Map 返回从 map 集合创建的属性列表，保存的是 map 深拷贝后的值。
func Map(m map[string]interface{}) Properties {
	p := New()
	for k, v := range m {
		p.Set(k, v)
	}
	return p
}

// Load 从文件读取属性列表。
func Load(filename string) (Properties, error) {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return Read(b, filepath.Ext(filename))
}

// Read 从 []byte 读取属性列表，ext 是文件扩展名，如 .toml、.yaml 等。
func Read(b []byte, ext string) (Properties, error) {

	r, ok := readers[strings.ToLower(ext)]
	if !ok {
		return nil, fmt.Errorf("unsupported file type %s", ext)
	}

	m, err := r(b)
	if err != nil {
		return nil, err
	}

	p := New()
	for k, v := range m {
		p.Set(k, v)
	}
	return p, nil
}

func (p *properties) Keys() []string {
	ret := make([]string, 0, len(p.m))
	for k := range p.m {
		ret = append(ret, k)
	}
	return ret
}

func (p *properties) Has(key string) bool {
	key = strings.ToLower(key)
	key = strings.TrimPrefix(key, "$.")
	_, ok := p.m[key]
	return ok
}

func (p *properties) Get(key string, opts ...GetOption) string {

	arg := getArg{}
	for _, opt := range opts {
		opt(&arg)
	}

	key = strings.ToLower(key)
	key = strings.TrimPrefix(key, "$.")
	val, ok := p.m[key]
	if !ok {
		val = cast.ToString(arg.def)
	}
	return val
}

func (p *properties) Set(key string, val interface{}) {
	key = strings.ToLower(key)
	switch v := reflect.ValueOf(val); v.Kind() {
	case reflect.Map:
		for _, k := range v.MapKeys() {
			mapValue := v.MapIndex(k).Interface()
			mapKey := cast.ToString(k.Interface())
			p.Set(key+"."+mapKey, mapValue)
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			subKey := fmt.Sprintf("%s[%d]", key, i)
			subValue := v.Index(i).Interface()
			p.Set(subKey, subValue)
		}
	case reflect.Struct:
		panic(util.UnimplementedMethod)
	default:
		p.m[key] = cast.ToString(val)
	}
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
func Bind(p Properties, i interface{}, opts ...BindOption) error {

	var v reflect.Value

	switch i.(type) {
	case reflect.Value:
		v = i.(reflect.Value)
	default:
		if v = reflect.ValueOf(i); v.Kind() != reflect.Ptr {
			return errors.New("i 必须是一个指针")
		}
		v = v.Elem()
	}

	arg := bindArg{}
	Key(RootKey)(&arg)

	for _, opt := range opts {
		opt(&arg)
	}

	t := v.Type()
	s := t.Name()
	if s == "" {
		switch t.Kind() {
		case reflect.Map, reflect.Slice, reflect.Array:
			s = t.Elem().Name()
			if s == "" {
				s = t.Elem().String()
			}
		default:
			s = t.String()
		}
	}

	return bind(p, v, arg.tag, bindOption{path: s})
}

// Resolve 解析 ${key:=def} 字符串，返回 key 对应的属性值，如果没有找到则返回
// def 值，如果 def 存在引用关系则递归解析直到获取最终的属性值。
func Resolve(p Properties, s string) (string, error) {
	return resolveString(p, s)
}

// GroupKeys 对属性列表的 key 按照 prefix 作为前缀进行分组，然后返回分组的名称。
func GroupKeys(p Properties, prefix string) []string {

	matches := func(key, prefix string) (string, bool) {
		if prefix == RootKey {
			return key, true
		}
		if !strings.HasPrefix(key, prefix+".") {
			return "", false
		}
		return strings.TrimPrefix(key, prefix+"."), true
	}

	var ret []string
	for _, key := range p.Keys() {
		s, ok := matches(key, prefix)
		if !ok {
			continue
		}
		k := strings.Split(s, ".")[0]
		if contain.Strings(ret, k) >= 0 {
			continue
		}
		ret = append(ret, k)
	}
	return ret
}
