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
	"io"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-spring/spring-logger"
	"github.com/go-spring/spring-utils"
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

// defaultProperties Properties 的默认实现
type defaultProperties struct {
	properties map[string]interface{}
	converters map[reflect.Type]Converter
}

// New defaultProperties 的构造函数
func New() *defaultProperties {

	p := &defaultProperties{
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
func (p *defaultProperties) Load(filename string) error {
	SpringLogger.Debug("load properties from file: ", filename)

	return p.read(func(v *viper.Viper) error {
		v.SetConfigFile(filename)
		return v.ReadInConfig()
	})
}

// Read 读取属性配置，支持 properties、yaml 和 toml 三种文件格式。
func (p *defaultProperties) Read(reader io.Reader, configType string) error {
	SpringLogger.Debug("load properties from reader type: ", configType)

	return p.read(func(v *viper.Viper) error {
		v.SetConfigType(configType)
		return v.ReadConfig(reader)
	})
}

func (p *defaultProperties) read(reader func(*viper.Viper) error) error {

	v := viper.New()
	if err := reader(v); err != nil {
		return err
	}

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.Set(key, val)
		SpringLogger.Tracef("%s=%v", key, val)
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

	return SpringUtils.IsValueType(t.Out(0).Kind()) && t.Out(1) == errorType
}

// Convert 添加类型转换器
func (p *defaultProperties) Convert(fn Converter) error {
	if t := reflect.TypeOf(fn); validConverter(t) {
		p.converters[t.Out(0)] = fn
		return nil
	}
	return errors.New("fn must be func(string)(type,error)")
}

// Converters 返回类型转换器集合
func (p *defaultProperties) Converters() map[reflect.Type]Converter {
	return p.converters
}

// Has 查询属性值是否存在，属性名称统一转成小写。
func (p *defaultProperties) Has(key string) bool {
	_, ok := p.properties[strings.ToLower(key)]
	return ok
}

// Get 返回属性值，不能存在返回 nil，属性名称统一转成小写。
func (p *defaultProperties) Get(key string) interface{} {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v
	}
	return nil
}

// GetFirst 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetFirst(keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := p.properties[strings.ToLower(key)]; ok {
			return v
		}
	}
	return nil
}

// GetDefault 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *defaultProperties) GetDefault(key string, def interface{}) interface{} {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v
	}
	return def
}

// Set 设置属性值，属性名称统一转成小写。
func (p *defaultProperties) Set(key string, value interface{}) {
	p.properties[strings.ToLower(key)] = value
}

// Keys 返回所有键，属性名称统一转成小写。
func (p *defaultProperties) Keys() []string {
	var keys []string
	for k := range p.properties {
		keys = append(keys, k)
	}
	return keys
}

// Range 遍历所有的属性值，属性名称统一转成小写。
func (p *defaultProperties) Range(fn func(string, interface{})) {
	for key, val := range p.properties {
		fn(key, val)
	}
}

// Fill 返回所有的属性值，属性名称统一转成小写。
func (p *defaultProperties) Fill(properties map[string]interface{}) {
	for key, val := range p.properties {
		properties[key] = val
	}
}

// Prefix 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *defaultProperties) Prefix(key string) map[string]interface{} {
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
func (p *defaultProperties) Group(key string) map[string]map[string]interface{} {
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

// BindOption 属性值绑定可选项
type BindOption struct {
	PrefixName string // 属性名前缀
	FullName   string // 完整属性名
	FieldName  string // 结构体字段的名称
}

// BindStruct 对结构体进行属性值绑定
func BindStruct(p Properties, v reflect.Value, opt BindOption) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)

		// 可能会开放私有字段
		fv = SpringUtils.PatchValue(fv, true)
		subFieldName := opt.FieldName + ".$" + ft.Name

		// 字段的绑定可选项
		subOpt := BindOption{
			PrefixName: opt.PrefixName,
			FullName:   opt.FullName,
			FieldName:  subFieldName,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			if err := BindStructField(p, fv, tag, subOpt); err != nil {
				return err
			}
			continue
		}

		// 匿名嵌套需要处理，不是结构体的具名字段无需处理
		if ft.Anonymous || ft.Type.Kind() == reflect.Struct {
			if err := BindStruct(p, fv, subOpt); err != nil {
				return err
			}
		}
	}
	return nil
}

// parsePropertyTag 解析属性值标签
func parsePropertyTag(str string) (key string, def interface{}) {
	ss := strings.SplitN(str, ":=", 2)
	if len(ss) > 1 {
		def = ss[1]
	}
	key = ss[0]
	return
}

// BindStructField 对结构体的字段进行属性绑定
func BindStructField(p Properties, v reflect.Value, str string, opt BindOption) error {

	// 检查 tag 语法是否正确
	if !(strings.HasPrefix(str, "${") && strings.HasSuffix(str, "}")) {
		return fmt.Errorf("%s 属性绑定的语法发生错误", opt.FieldName)
	}

	// 指针不能作为属性绑定的目标
	if v.Kind() == reflect.Ptr {
		return fmt.Errorf("%s 属性绑定的目标不能是指针", opt.FieldName)
	}

	key, def := parsePropertyTag(str[2 : len(str)-1])

	// 此处使用最短属性名
	if opt.FullName == "" {
		opt.FullName = key
	} else if key != "" {
		opt.FullName = opt.FullName + "." + key
	}

	// 属性名如果有前缀要加上前缀
	if opt.PrefixName != "" {
		key = opt.PrefixName + "." + key
	}

	return BindValue(p, v, key, def, opt)
}

// ResolveProperty 解析属性值，查看其是否具有引用关系
func ResolveProperty(p Properties, _ string, value interface{}) (interface{}, error) {
	str, ok := value.(string)

	// 不是字符串或者没有使用配置引用语法
	if !ok || !strings.HasPrefix(str, "${") {
		return value, nil
	}

	key, def := parsePropertyTag(str[2 : len(str)-1])
	if val := p.GetDefault(key, def); val != nil {
		return ResolveProperty(p, key, val)
	}

	return nil, fmt.Errorf("property \"%s\" not config", key)
}

func getPropertyValue(p Properties, kind reflect.Kind, key string, def interface{}, opt BindOption) (interface{}, error) {

	// 首先获取精确匹配的属性值
	if val := p.Get(key); val != nil {
		return val, nil
	}

	// Map 和 Struct 类型获取具有相同前缀的属性值
	if kind == reflect.Map || kind == reflect.Struct {
		if prefixValue := p.Prefix(key); len(prefixValue) > 0 {
			return prefixValue, nil
		}
	}

	// 最后使用默认值，需要解析配置引用语法
	if def != nil {
		return ResolveProperty(p, key, def)
	}

	return nil, fmt.Errorf("%s properties \"%s\" not config", opt.FieldName, opt.FullName)
}

// BindValue 对任意 value 进行属性绑定
func BindValue(p Properties, v reflect.Value, key string, def interface{}, opt BindOption) error {

	t := v.Type()
	k := t.Kind()

	// 存在值类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := p.Converters()[t]; ok {
		propValue, err := getPropertyValue(p, k, key, def, opt)
		if err == nil {
			fnValue := reflect.ValueOf(fn)
			out := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
			v.Set(out[0])
		}
		return err
	}

	if k == reflect.Struct {
		if def == nil {
			return BindStruct(p, v, BindOption{
				PrefixName: key,
				FullName:   opt.FullName,
				FieldName:  opt.FieldName,
			})
		} else { // 前面已经校验过是否存在值类型转换器
			return fmt.Errorf("%s 结构体字段不能指定默认值", opt.FieldName)
		}
	}

	propValue, err := getPropertyValue(p, k, key, def, opt)
	if err != nil {
		return err
	}

	switch k {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		if u, err := cast.ToUint64E(propValue); err == nil {
			v.SetUint(u)
		} else {
			return fmt.Errorf("property value %s isn't uint type", opt.FullName)
		}
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		if i, err := cast.ToInt64E(propValue); err == nil {
			v.SetInt(i)
		} else {
			return fmt.Errorf("property value %s isn't int type", opt.FullName)
		}
	case reflect.Float64, reflect.Float32:
		if f, err := cast.ToFloat64E(propValue); err == nil {
			v.SetFloat(f)
		} else {
			return fmt.Errorf("property value %s isn't float type", opt.FullName)
		}
	case reflect.String:
		if s, err := cast.ToStringE(propValue); err == nil {
			v.SetString(s)
		} else {
			return fmt.Errorf("property value %s isn't string type", opt.FullName)
		}
	case reflect.Bool:
		if b, err := cast.ToBoolE(propValue); err == nil {
			v.SetBool(b)
		} else {
			return fmt.Errorf("property value %s isn't bool type", opt.FullName)
		}
	case reflect.Slice:
		elemType := v.Type().Elem()
		elemKind := elemType.Kind()

		// 如果是字符串的话，尝试按照逗号进行切割
		if s, ok := propValue.(string); ok {
			propValue = strings.Split(s, ",")
		}

		// 处理使用类型转换器的场景
		if fn, ok := p.Converters()[elemType]; ok {
			if s0, err := cast.ToStringSliceE(propValue); err == nil {
				sv := reflect.MakeSlice(t, len(s0), len(s0))
				fnValue := reflect.ValueOf(fn)
				for i, iv := range s0 {
					res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
					sv.Index(i).Set(res[0])
				}
				v.Set(sv)
				return nil
			} else {
				return fmt.Errorf("property value %s isn't []string type", opt.FullName)
			}
		}

		switch elemKind {
		case reflect.Uint64:
			if i, err := ToUint64SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint64 type", opt.FullName)
			}
		case reflect.Uint32:
			if i, err := ToUint32SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint32 type", opt.FullName)
			}
		case reflect.Uint16:
			if i, err := ToUint16SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint16 type", opt.FullName)
			}
		case reflect.Uint8:
			if i, err := ToUint8SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint8 type", opt.FullName)
			}
		case reflect.Uint:
			if i, err := ToUintSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint type", opt.FullName)
			}
		case reflect.Int64:
			if i, err := ToInt64SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int64 type", opt.FullName)
			}
		case reflect.Int32:
			if i, err := ToInt32SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int32 type", opt.FullName)
			}
		case reflect.Int16:
			if i, err := ToInt16SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int16 type", opt.FullName)
			}
		case reflect.Int8:
			if i, err := ToInt8SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int8 type", opt.FullName)
			}
		case reflect.Int:
			if i, err := ToIntSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int type", opt.FullName)
			}
		case reflect.Float64, reflect.Float32:
			return errors.New("暂未支持")
		case reflect.String:
			if i, err := cast.ToStringSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []string type", opt.FullName)
			}
		case reflect.Bool:
			if b, err := cast.ToBoolSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(b))
			} else {
				return fmt.Errorf("property value %s isn't []bool type", opt.FullName)
			}
		default:
			// 处理结构体字段的场景
			if s, ok := propValue.([]interface{}); ok {
				result := reflect.MakeSlice(t, len(s), len(s))
				for i, si := range s {
					if sv, err := cast.ToStringMapE(si); err == nil {
						ev := reflect.New(elemType)
						subFullName := fmt.Sprintf("%s[%d]", key, i)
						err = BindStruct(&defaultProperties{sv, p.Converters()}, ev.Elem(), BindOption{
							FullName:  subFullName,
							FieldName: opt.FieldName,
						})
						if err != nil {
							return err
						}
						result.Index(i).Set(ev.Elem())
					} else {
						return fmt.Errorf("property value %s isn't []map[string]interface{}", opt.FullName)
					}
				}
				v.Set(result)
			} else {
				return fmt.Errorf("property value %s isn't []map[string]interface{}", opt.FullName)
			}
		}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return fmt.Errorf("field: %s isn't map[string]interface{}", opt.FieldName)
		}

		elemType := t.Elem()
		elemKind := elemType.Kind()

		// 首先处理使用类型转换器的场景
		if fn, ok := p.Converters()[elemType]; ok {
			if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
				prefix := key + "."
				fnValue := reflect.ValueOf(fn)
				result := reflect.MakeMap(t)
				for k0, v0 := range mapValue {
					res := fnValue.Call([]reflect.Value{reflect.ValueOf(v0)})
					k0 = strings.TrimPrefix(k0, prefix)
					result.SetMapIndex(reflect.ValueOf(k0), res[0])
				}
				v.Set(result)
				return nil
			} else {
				return fmt.Errorf("property value %s isn't map[string]string", opt.FullName)
			}
		}

		switch elemKind {
		case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
			return errors.New("暂未支持")
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
			return errors.New("暂未支持")
		case reflect.Float64, reflect.Float32:
			return errors.New("暂未支持")
		case reflect.Bool:
			return errors.New("暂未支持")
		case reflect.String:
			if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
				prefix := key + "."
				result := make(map[string]string)
				for k0, v0 := range mapValue {
					k0 = strings.TrimPrefix(k0, prefix)
					result[k0] = v0
				}
				v.Set(reflect.ValueOf(result))
			} else {
				return fmt.Errorf("property value %s isn't map[string]string", opt.FullName)
			}
		default:
			// 处理结构体字段的场景
			if mapValue, err := cast.ToStringMapE(propValue); err == nil {
				temp := make(map[string]map[string]interface{})
				trimKey := key + "."
				var ok bool

				// 将一维 map 变成二维 map
				for k0, v0 := range mapValue {
					k0 = strings.TrimPrefix(k0, trimKey)
					sk := strings.Split(k0, ".")
					var item map[string]interface{}
					if item, ok = temp[sk[0]]; !ok {
						item = make(map[string]interface{})
						temp[sk[0]] = item
					}
					item[sk[1]] = v0
				}

				result := reflect.MakeMapWithSize(t, len(temp))
				for k1, v1 := range temp {
					ev := reflect.New(elemType)
					subFullName := fmt.Sprintf("%s.%s", key, k1)
					err = BindStruct(&defaultProperties{v1, p.Converters()}, ev.Elem(), BindOption{
						FullName:  subFullName,
						FieldName: opt.FieldName,
					})
					if err != nil {
						return err
					}
					result.SetMapIndex(reflect.ValueOf(k1), ev.Elem())
				}

				v.Set(result)
			} else {
				return fmt.Errorf("property value %s isn't map[string]map[string]interface{}", opt.FullName)
			}
		}
	default:
		return errors.New(opt.FieldName + " unsupported type " + v.Kind().String())
	}
	return nil
}

// Bind 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) Bind(key string, i interface{}) error {

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		return errors.New("参数 v 必须是一个指针")
	}

	t := v.Type().Elem()
	s := t.Name() // 当绑定对象是 map 或者 slice 时，取元素的类型名
	if s == "" && (t.Kind() == reflect.Map || t.Kind() == reflect.Slice) {
		s = t.Elem().Name()
	}

	return BindValue(p, v.Elem(), key, nil, BindOption{FieldName: s, FullName: key})
}
