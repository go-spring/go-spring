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

// Converter converts string value into user-defined value. It should
// be function type, and its prototype is func(string)(type,error).
type Converter interface{}

// Splitter splits string value into []string value.
type Splitter func(string) ([]string, error)

// Reader parses []byte value into nested map[string]interface{}.
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

	// Converts string value into time.Time value. The string value
	// may have its own time format after >> splitter, otherwise it
	// uses a default time format 2006-01-02 15:04:05 -0700.
	RegisterConverter(func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		format := "2006-01-02 15:04:05 -0700"
		if ss := strings.Split(s, ">>"); len(ss) == 2 {
			format = strings.TrimSpace(ss[1])
			s = strings.TrimSpace(ss[0])
		}
		return cast.ToTimeE(s, format)
	})

	// Converts string value into time.Duration value. The string value
	// should have its own time unit such as "ns", "ms", "s", "m", etc.
	RegisterConverter(func(s string) (time.Duration, error) {
		return cast.ToDurationE(s)
	})
}

// RegisterReader registers Reader for some file extensions.
func RegisterReader(r Reader, ext ...string) {
	for _, s := range ext {
		readers[s] = r
	}
}

// RegisterSplitter registers Splitter and named it.
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

// RegisterConverter registers Converter for non-primitive type such as
// time.Time, time.Duration, or other user-defined value type.
func RegisterConverter(fn Converter) {
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

// Has returns whether key exists.
func (p *Properties) Has(key string) bool {
	keyPath, err := SplitPath(key)
	if err != nil {
		return false
	}
	t := p.tree
	for i, s := range keyPath {
		v, ok := t[s]
		if !ok {
			return false
		}
		_, ok = v.(struct{})
		if ok {
			return i == len(keyPath)-1
		}
		t = v.(map[string]interface{})
	}
	return true
}

type getArg struct {
	def string
}

type GetOption func(arg *getArg)

// Def returns v when key not exits.
func Def(v string) GetOption {
	return func(arg *getArg) {
		arg.def = v
	}
}

// Get returns key's value, using Def to return a default value.
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

// Set sets key's value to be a primitive type as int or string,
// or a slice or map nested with primitive type elements. One thing
// you should know is Set actions as overlap but not replace, that
// means when you set a slice or a map, an existing path will remain
// when it doesn't exist in the slice or map even they share a same
// prefix path.
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
			mapKey := cast.ToString(k.Interface())
			mapValue := v.MapIndex(k).Interface()
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
			err = p.Set(subKey, subValue)
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

// Resolve resolves string value that contains references to other
// properties, the references are defined by ${key:=def}.
func (p *Properties) Resolve(s string) (string, error) {
	return resolveString(p, s)
}

type bindArg struct {
	tag string
}

type BindOption func(arg *bindArg)

// Key binds properties using one key.
func Key(key string) BindOption {
	return func(arg *bindArg) {
		arg.tag = "${" + key + "}"
	}
}

// Tag binds properties using one tag.
func Tag(tag string) BindOption {
	return func(arg *bindArg) {
		arg.tag = tag
	}
}

// Bind binds properties to a value, the bind value can be primitive type,
// map, slice, struct. When binding to struct, the tag 'value' indicates
// which properties should be bind. The 'value' tags are defined by
// value:"${a:=b|splitter}", 'a' is the key, 'b' is the default value,
// 'splitter' is the Splitter's name when you want split string value
// into []string value.
func (p *Properties) Bind(i interface{}, opts ...BindOption) error {

	var v reflect.Value
	{
		switch e := i.(type) {
		case reflect.Value:
			v = e
		default:
			v = reflect.ValueOf(i)
			if v.Kind() != reflect.Ptr {
				return errors.New("i should be a ptr")
			}
			v = v.Elem()
		}
	}

	arg := bindArg{tag: "${ROOT}"}
	for _, opt := range opts {
		opt(&arg)
	}

	t := v.Type()
	typeName := t.Name()
	if typeName == "" { // primitive type has no name
		typeName = t.String()
	}

	param := BindParam{
		Type: t,
		Path: typeName,
	}
	err := param.BindTag(arg.tag)
	if err != nil {
		return err
	}
	return BindValue(p, v, param)
}

// SplitPath splits the key into individual parts.
func SplitPath(key string) ([]string, error) {
	if len(key) == 0 {
		return nil, fmt.Errorf("error key '%s'", key)
	}
	var (
		keyPath     []string
		lastChar    int32
		lastIndex   int
		leftBracket bool
	)
	for i, c := range key {
		switch c {
		case ' ':
			return nil, fmt.Errorf("error key '%s'", key)
		case '.':
			if leftBracket {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			if lastChar == ']' {
				lastIndex = i + 1
				lastChar = c
				continue
			}
			if lastIndex == i {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, key[lastIndex:i])
			lastIndex = i + 1
			lastChar = c
		case '[':
			if leftBracket {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			if i == 0 || lastChar == ']' || lastChar == '.' {
				lastIndex = i + 1
				leftBracket = true
				lastChar = c
				continue
			}
			if lastIndex == i {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, key[lastIndex:i])
			lastIndex = i + 1
			leftBracket = true
			lastChar = c
		case ']':
			if !leftBracket || lastIndex == i {
				return nil, fmt.Errorf("error key '%s'", key)
			}
			keyPath = append(keyPath, key[lastIndex:i])
			lastIndex = i + 1
			leftBracket = false
			lastChar = c
		default:
			lastChar = c
		}
	}
	if leftBracket || lastChar == '.' {
		return nil, fmt.Errorf("error key '%s'", key)
	}
	if lastChar != ']' {
		keyPath = append(keyPath, key[lastIndex:])
	}
	return keyPath, nil
}

// checkKey checks whether the key exists, if the key is collection then it will be
// stored as map[string]interface{}, otherwise it will be stored as struct{}.
func (p *Properties) checkKey(key string, collection bool) (exist bool, err error) {
	keyPath, err := SplitPath(key)
	if err != nil {
		return false, err
	}
	t := p.tree
	exist = true
	for i, s := range keyPath {
		v, ok := t[s]
		if !ok {
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
		_, ok = v.(map[string]interface{})
		if ok {
			if i < len(keyPath)-1 || collection {
				t = v.(map[string]interface{})
				continue
			}
			err = fmt.Errorf("property %q want a value but has sub keys %v", key, v)
			return
		}
		_, ok = v.(struct{})
		if ok {
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
