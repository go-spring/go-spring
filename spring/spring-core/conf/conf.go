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

// Package conf reads configuration from many format file, such as Java
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
	"github.com/go-spring/spring-core/conf/internal"
	"github.com/go-spring/spring-core/conf/prop"
	"github.com/go-spring/spring-core/conf/toml"
	"github.com/go-spring/spring-core/conf/yaml"
)

// Splitter splits string into []string by some characters.
type Splitter func(string) ([]string, error)

// Reader parses []byte into nested map[string]interface{}.
type Reader func(b []byte) (map[string]interface{}, error)

var (
	readers    = map[string]Reader{}
	splitters  = map[string]Splitter{}
	converters = map[reflect.Type]util.Converter{}
)

func init() {

	RegisterReader(prop.Read, ".properties")
	RegisterReader(yaml.Read, ".yaml", ".yml")
	RegisterReader(toml.Read, ".toml", ".tml")

	// converts string into time.Time. The string value may have its own
	// time format defined after >> splitter, otherwise it uses a default
	// time format `2006-01-02 15:04:05 -0700`.
	RegisterConverter(func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		format := "2006-01-02 15:04:05 -0700"
		if ss := strings.Split(s, ">>"); len(ss) == 2 {
			format = strings.TrimSpace(ss[1])
			s = strings.TrimSpace(ss[0])
		}
		return cast.ToTimeE(s, format)
	})

	// converts string into time.Duration. The string should have its own
	// time unit such as "ns", "ms", "s", "m", etc.
	RegisterConverter(func(s string) (time.Duration, error) {
		return cast.ToDurationE(s)
	})
}

// RegisterReader registers its Reader for some kind of file extension.
func RegisterReader(r Reader, ext ...string) {
	for _, s := range ext {
		readers[s] = r
	}
}

// RegisterSplitter registers a Splitter and named it.
func RegisterSplitter(name string, fn Splitter) {
	splitters[name] = fn
}

// RegisterConverter registers its converter for non-primitive type such as
// time.Time, time.Duration, or other user-defined value type.
func RegisterConverter(fn util.Converter) {
	t := reflect.TypeOf(fn)
	if !util.IsValidConverter(t) {
		panic(errors.New("converter should be func(string)(type,error)"))
	}
	converters[t.Out(0)] = fn
}

// Properties stores the data with map[string]string and the keys are case-sensitive,
// you can get one of them by its key, or bind some of them to a value.
// There are too many formats of configuration files, and too many conflicts between
// them. Each format of configuration file provides its special characteristics, but
// usually they are not all necessary, and complementary. For example, `conf` disabled
// Java properties' expansion when reading file, but also provides similar function
// when getting or binding properties.
// A good rule of thumb is that treating application configuration as a tree, but not
// all formats of configuration files designed as a tree or not ideal, for instance
// Java properties isn't strictly verified. Although configuration can store as a tree,
// but it costs more CPU time when getting properties because it reads property node
// by node. So `conf` uses a tree to strictly verify and a flat map to store.
type Properties struct {
	storage *internal.Storage
}

// New creates empty *Properties.
func New() *Properties {
	return &Properties{
		storage: internal.NewStorage(),
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

func (p *Properties) Copy() *Properties {
	return &Properties{
		storage: p.storage.Copy(),
	}
}

// Keys returns all sorted keys.
func (p *Properties) Keys() []string {
	return p.storage.Keys()
}

// Has returns whether key exists.
func (p *Properties) Has(key string) bool {
	return p.storage.Has(key)
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
	val := p.storage.Get(key)
	if val != "" {
		return val
	}
	arg := getArg{}
	for _, opt := range opts {
		opt(&arg)
	}
	return arg.def
}

func Flatten(key string, val interface{}, result map[string]string) error {
	switch v := reflect.ValueOf(val); v.Kind() {
	case reflect.Map:
		if v.Len() == 0 {
			result[key] = ""
			return nil
		}
		for _, k := range v.MapKeys() {
			mapKey := cast.ToString(k.Interface())
			mapValue := v.MapIndex(k).Interface()
			err := Flatten(key+"."+mapKey, mapValue, result)
			if err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		if v.Len() == 0 {
			result[key] = ""
			return nil
		}
		for i := 0; i < v.Len(); i++ {
			subKey := fmt.Sprintf("%s[%d]", key, i)
			subValue := v.Index(i).Interface()
			err := Flatten(subKey, subValue, result)
			if err != nil {
				return err
			}
		}
	default:
		result[key] = cast.ToString(val)
	}
	return nil
}

// Set sets key's value to be a primitive type as int or string,
// or a slice or map nested with primitive type elements. One thing
// you should know is Set actions as overlap but not replace, that
// means when you set a slice or a map, an existing path will remain
// when it doesn't exist in the slice or map even they share a same
// prefix path.
func (p *Properties) Set(key string, val interface{}) error {
	if key == "" {
		return nil
	}
	m := make(map[string]string)
	err := Flatten(key, val, m)
	if err != nil {
		return err
	}
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		err = p.storage.Set(k, m[k])
		if err != nil {
			return err
		}
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
		Path: typeName,
	}
	err := param.BindTag(arg.tag, "")
	if err != nil {
		return err
	}
	return BindValue(p, v, t, param, nil)
}
