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

// Package conf reads configuration from any format file, including Java
// properties, yaml, toml, etc.
package conf

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-spring/spring-base/cast"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf/prop"
	"github.com/go-spring/spring-core/conf/toml"
	"github.com/go-spring/spring-core/conf/yaml"
)

// Converter 类型转换器，函数原型为 func(string)(type,error)。
type Converter interface{}

// Splitter 字符串分割器，用于将字符串按逗号分割成字符串切片。
type Splitter func(string) ([]string, error)

// Reader 属性列表解析器，将字节数组解析成 map 数据。
type Reader func(b []byte) (map[string]interface{}, error)

var (
	readers    = map[string]Reader{}
	splitters  = map[string]Splitter{}
	converters = map[reflect.Type]Converter{}
)

func init() {

	RegisterReader(prop.Read, ".properties")
	RegisterReader(yaml.Read, ".yaml", ".yml")
	RegisterReader(toml.Read, ".toml", ".tml")

	// 日期转换函数，支持时间戳格式，支持日期字符串(日期字符串>>日期字符串的格式)。
	RegisterConverter(func(s string) (time.Time, error) {
		format := "2006-01-02 15:04:05 -0700"
		if ss := strings.Split(s, ">>"); len(ss) == 2 {
			format = strings.TrimSpace(ss[1])
			s = strings.TrimSpace(ss[0])
		}
		return cast.ToTimeE(s, format)
	})

	// 时长转换函数，支持 "ns", "us" (or "µs"), "ms", "s", "m", "h" 等。
	RegisterConverter(func(s string) (time.Duration, error) {
		return cast.ToDurationE(s)
	})
}

// RegisterReader 注册属性列表解析器，ext 是解析器支持的文件扩展名。
func RegisterReader(r Reader, ext ...string) {
	for _, s := range ext {
		readers[s] = r
	}
}

// RegisterSplitter 注册字符串分割器。
func RegisterSplitter(name string, fn Splitter) {
	splitters[name] = fn
}

func validConverter(t reflect.Type) bool {
	return t.Kind() == reflect.Func &&
		t.NumIn() == 1 &&
		t.In(0).Kind() == reflect.String &&
		t.NumOut() == 2 &&
		IsValueType(t.Out(0)) &&
		util.IsErrorType(t.Out(1))
}

// RegisterConverter 注册类型转换器。
func RegisterConverter(fn interface{}) {
	t := reflect.TypeOf(fn)
	if !validConverter(t) {
		panic(errors.New("fn must be func(string)(type,error)"))
	}
	converters[t.Out(0)] = fn
}

// Properties There are too many formats of configuration files, and too many
// conflicts between them. Each format of configuration file provides its special
// characteristics, but usually they are not all necessary, and complementary. For
// example, conf disabled Java properties' expansion when reading file, but it also
// provides similar function when getting properties.
// A good rule of thumb is that treating application configuration as a tree, but not
// all formats of configuration files designed like this or not ideal, such as Java
// properties which not strictly verified. Although configuration can store as a tree,
// but it costs more CPU time when getting properties because it reads property node
// by node. So conf uses a tree to strictly verify and a flat map to store.
//
//
//提供创建和读取属性列表的方法。它使用扁平的 map[string]string 结
// 构存储数据，属性的 key 可以是 a.b.c 或者 a[0].b 两种形式，a.b.c 表示从 map
// 结构中获取属性值，a[0].b 表示从切片结构中获取属性值，并且 key 是大小写敏感的。
type Properties struct {
	data map[string]string      // stores key and value.
	tree map[string]interface{} // stores split key path.
}

// New creates empty *Properties.
func New() *Properties {
	return &Properties{
		data: make(map[string]string),
		tree: make(map[string]interface{}),
	}
}

// Map creates *Properties from map.
func Map(m map[string]interface{}) (*Properties, error) {
	p := New()
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := p.Set(k, m[k]); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// Load creates *Properties from file.
func Load(file string) (*Properties, error) {
	p := New()
	if err := p.Load(file); err != nil {
		return nil, err
	}
	return p, nil
}

// Load loads properties from file.
func (p *Properties) Load(file string) error {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return p.Bytes(b, filepath.Ext(file))
}

// Read creates *Properties from io.Reader, ext is the file name extension.
func Read(r io.Reader, ext string) (*Properties, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Bytes(b, ext)
}

// Bytes creates *Properties from []byte, ext is the file name extension.
func Bytes(b []byte, ext string) (*Properties, error) {
	p := New()
	if err := p.Bytes(b, ext); err != nil {
		return nil, err
	}
	return p, nil
}

// Bytes loads properties from []byte, ext is the file name extension.
func (p *Properties) Bytes(b []byte, ext string) error {
	r, ok := readers[ext]
	if !ok {
		return fmt.Errorf("unsupported file type %s", ext)
	}
	m, err := r(b)
	if err != nil {
		return err
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err = p.Set(k, m[k]); err != nil {
			return err
		}
	}
	return nil
}

// Keys returns all sorted keys.
func (p *Properties) Keys() []string {
	keys := make([]string, 0, len(p.data))
	for k := range p.data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func splitPath(key string) ([]string, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("error key '%s'", key)
	}
	var (
		keyPath      []string
		lastIndex    int
		leftBracket  bool
		rightBracket bool
	)
	for i, c := range key {
		switch c {
		case '.':
			s := strings.TrimSpace(key[lastIndex:i])
			if s == "" {
				if rightBracket {
					lastIndex = i + 1
					rightBracket = false
					continue
				}
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, s)
			lastIndex = i + 1
			rightBracket = false
		case '[':
			if leftBracket {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			s := strings.TrimSpace(key[lastIndex:i])
			if s == "" {
				if len(keyPath) == 0 || rightBracket {
					lastIndex = i + 1
					leftBracket = true
					rightBracket = false
					continue
				}
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, s)
			lastIndex = i + 1
			leftBracket = true
			rightBracket = false
		case ']':
			if !leftBracket {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			s := strings.TrimSpace(key[lastIndex:i])
			if s == "" {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, s)
			lastIndex = i + 1
			leftBracket = false
			rightBracket = true
		}
	}
	if lastIndex < len(key) {
		s := strings.TrimSpace(key[lastIndex:])
		if s == "" {
			return nil, fmt.Errorf("error key '%s'", key)
		}
		keyPath = append(keyPath, s)
	}
	return keyPath, nil
}

// checkKey 检查属性 key 是否合法，collection 表示是否为空的集合数据。
func (p *Properties) checkKey(key string, collection bool) (exist bool, err error) {

	var (
		ok bool
		v  interface{}
		t  map[string]interface{}
	)

	t = p.tree
	exist = true
	keyPath, err := splitPath(key)
	if err != nil {
		return false, err
	}
	for i, s := range keyPath {

		if v, ok = t[s]; !ok {
			if i < len(keyPath)-1 || collection {
				m := make(map[string]interface{})
				t[s] = m
				t = m
			} else {
				t[s] = struct{}{}
			}
			exist = false
			continue
		}

		if _, ok = v.(map[string]interface{}); ok {
			if i < len(keyPath)-1 || collection {
				t = v.(map[string]interface{})
				continue
			}
			err = fmt.Errorf("property %q want a value but has sub keys %v", key, v)
			return
		}

		if _, ok = v.(struct{}); ok {
			if i == len(keyPath)-1 && !collection {
				continue
			}
			oldKey := strings.Join(keyPath[:i+1], ".")
			err = fmt.Errorf("property %q has a value but want another sub key %q", oldKey, key)
			return
		}
	}
	return
}

// Has 返回属性 key 是否存在。
func (p *Properties) Has(key string) bool {

	var (
		ok bool
		v  interface{}
		t  map[string]interface{}
	)

	t = p.tree
	keyPath, err := splitPath(key)
	if err != nil {
		return false
	}
	for i, s := range keyPath {
		if v, ok = t[s]; !ok {
			if i < len(keyPath)-1 {
				return false
			}
			if _, ok = t[s+"[0]"]; !ok {
				return false
			}
			return true
		}
		if _, ok = v.(struct{}); ok {
			if i < len(keyPath)-1 {
				return false
			}
			return true
		}
		t = v.(map[string]interface{})
	}
	return true
}

type getArg struct {
	def string
}

type GetOption func(arg *getArg)

// Def 为 Get 方法设置默认值。
func Def(v string) GetOption {
	return func(arg *getArg) {
		arg.def = v
	}
}

// Get 获取 key 对应的属性值，注意 key 是大小写敏感的。当 key 对应的属性值存在时，
// 或者 key 对应的属性值不存在但设置了默认值时，Get 方法返回 string 类型的数据，
// 当 key 对应的属性值不存在且没有设置默认值时 Get 方法返回 nil。因此可以通过判断
// Get 方法的返回值是否为 nil 来判断 key 对应的属性值是否存在。
func (p *Properties) Get(key string, opts ...GetOption) string {
	if val, ok := p.data[key]; ok {
		return val
	}
	arg := getArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	return arg.def
}

// Set 设置 key 对应的属性值，如果 key 对应的属性值已经存在则 Set 方法会覆盖旧
// 值。Set 方法除了支持 string 类型的属性值，还支持 int、uint、bool 等其他基础
// 数据类型的属性值。特殊情况下，Set 方法也支持 slice 、map 与基础数据类型组合构
// 成的属性值，其处理方式是将组合结构层层展开，可以将组合结构看成一棵树，那么叶子结
// 点的路径就是属性的 key，叶子结点的值就是属性的值。注意: conf 的配置文件是补充
// 关系，而不是替换关系，这一条原则我也经常会搞混，尤其在和其他配置库相比较的时候。
func (p *Properties) Set(key string, val interface{}) error {
	switch v := reflect.ValueOf(val); v.Kind() {
	case reflect.Map:
		exist, err := p.checkKey(key, true)
		if err != nil {
			return err
		}
		if v.Len() == 0 && !exist {
			p.data[key] = ""
			return nil
		}
		for _, k := range v.MapKeys() {
			mapValue := v.MapIndex(k).Interface()
			mapKey := cast.ToString(k.Interface())
			err = p.Set(key+"."+mapKey, mapValue)
			if err != nil {
				return err
			}
		}
		if _, ok := p.data[key]; ok {
			delete(p.data, key)
		}
	case reflect.Array, reflect.Slice:
		exist, err := p.checkKey(key, true)
		if err != nil {
			return err
		}
		if v.Len() == 0 && !exist {
			p.data[key] = ""
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			subKey := fmt.Sprintf("%s[%d]", key, i)
			subValue := v.Index(i).Interface()
			err := p.Set(subKey, subValue)
			if err != nil {
				return err
			}
		}
		if _, ok := p.data[key]; ok {
			delete(p.data, key)
		}
	default:
		_, err := p.checkKey(key, false)
		if err != nil {
			return err
		}
		p.data[key] = cast.ToString(val)
	}
	return nil
}

// Resolve 解析字符串中包含的所有属性引用即 ${key:=def} 的内容，并且支持递归引用。
func (p *Properties) Resolve(s string) (string, error) {
	return resolveString(p, s)
}

type bindArg struct {
	tag string
}

type BindOption func(arg *bindArg)

// Key 设置属性绑定使用的 key 。
func Key(key string) BindOption {
	return func(arg *bindArg) {
		arg.tag = "${" + key + "}"
	}
}

// Tag 设置属性绑定使用的 tag 。
func Tag(tag string) BindOption {
	return func(arg *bindArg) {
		arg.tag = tag
	}
}

// Bind 将 key 对应的属性值绑定到某个数据类型的实例上。i 必须是一个指针，只有这
// 样才能将修改传递出去。Bind 方法使用 tag 字符串对数据实例进行属性绑定，其语法
// 为 value:"${a:=b}"，其中 value 表示属性绑定，${} 表示属性引用，a 表示属性
// 的名称，:=b 表示为属性设置默认值。而且 tag 字符串还支持在默认值中进行嵌套引用
// ，即 ${a:=${b}}。当然，还有两点需要特别说明：
// 一是对 array、slice、map、struct 这些复合类型不能设置非空默认值，因为如果
// 默认值太长会影响阅读体验，而且解析起来也并不容易；
// 二是可以省略属性名而只有默认值，即 ${:=b}，原因是某些情况下属性名可能没想好或
// 者不太重要，比如，得益于字符串差值的实现，这种语法可以用于动态生成新的属性值，
// 也有人认为这是一种对 Golang 缺少默认值语法的补充，Bug is Feature。
func (p *Properties) Bind(i interface{}, opts ...BindOption) error {

	var v reflect.Value

	switch e := i.(type) {
	case reflect.Value:
		v = e
	default:
		if v = reflect.ValueOf(i); v.Kind() != reflect.Ptr {
			return errors.New("属性绑定的对象必须是一个指针")
		}
		v = v.Elem()
	}

	arg := bindArg{tag: "${}"}
	for _, opt := range opts {
		opt(&arg)
	}

	t := v.Type()
	typeName := t.Name()
	if typeName == "" { // 简单类型没有名字
		typeName = t.String()
	}

	param := BindParam{Type: t, Path: typeName}
	if err := param.BindTag(arg.tag); err != nil {
		return err
	}
	return BindValue(p, v, param)
}
