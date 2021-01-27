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

// defaultProperties Properties 的默认实现
type defaultProperties struct {
	properties map[string]interface{}
	converters map[reflect.Type]TypeConverter
}

// NewDefaultProperties defaultProperties 的构造函数
func NewDefaultProperties() *defaultProperties {

	p := &defaultProperties{
		properties: make(map[string]interface{}),
		converters: make(map[reflect.Type]TypeConverter),
	}

	// 注册时长转换函数 string -> time.Duration converter
	// time units are "ns", "us" (or "µs"), "ms", "s", "m", "h"。
	p.AddTypeConverter(func(s string) time.Duration {
		r, err := cast.ToDurationE(s)
		SpringUtils.Panic(err).When(err != nil)
		return r
	})

	// 注册日期转换函数 string -> time.Time converter
	// 支持非常多的日期格式，参见 cast.StringToDate。
	p.AddTypeConverter(func(s string) time.Time {
		r, err := cast.ToTimeE(s)
		SpringUtils.Panic(err).When(err != nil)
		return r
	})

	return p
}

func (p *defaultProperties) readProperties(reader func(*viper.Viper) error) {

	v := viper.New()
	err := reader(v)
	SpringUtils.Panic(err).When(err != nil)

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.SetProperty(key, val)
		SpringLogger.Tracef("%s=%v", key, val)
	}
}

// LoadProperties 加载属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *defaultProperties) LoadProperties(filename string) {
	SpringLogger.Debug("load properties from file: ", filename)

	p.readProperties(func(v *viper.Viper) error {
		v.SetConfigFile(filename)
		return v.ReadInConfig()
	})
}

// ReadProperties 读取属性配置文件，支持 properties、yaml 和 toml 三种文件格式。
func (p *defaultProperties) ReadProperties(reader io.Reader, configType string) {
	SpringLogger.Debug("load properties from reader type: ", configType)

	p.readProperties(func(v *viper.Viper) error {
		v.SetConfigType(configType)
		return v.ReadConfig(reader)
	})
}

// GetProperty 返回 keys 中第一个存在的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetProperty(keys ...string) interface{} {
	for _, key := range keys {
		if v, ok := p.properties[strings.ToLower(key)]; ok {
			return v
		}
	}
	return nil
}

// SetProperty 设置属性值，属性名称统一转成小写。
func (p *defaultProperties) SetProperty(key string, value interface{}) {
	p.properties[strings.ToLower(key)] = value
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值，属性名称统一转成小写。
func (p *defaultProperties) GetDefaultProperty(key string, def interface{}) (interface{}, bool) {
	if v, ok := p.properties[strings.ToLower(key)]; ok {
		return v, true
	}
	return def, false
}

// GetPrefixProperties 返回指定前缀的属性值集合，属性名称统一转成小写。
func (p *defaultProperties) GetPrefixProperties(prefix string) map[string]interface{} {
	prefix = strings.ToLower(prefix)
	result := make(map[string]interface{})
	for k, v := range p.properties {
		if k == prefix || strings.HasPrefix(k, prefix+".") {
			result[k] = v
		}
	}
	return result
}

// GetGroupedProperties 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func (p *defaultProperties) GetGroupedProperties(prefix string) map[string]map[string]interface{} {
	prefix = strings.ToLower(prefix) + "."
	result := make(map[string]map[string]interface{})
	for k, v := range p.properties {
		if strings.HasPrefix(k, prefix) { // 形如 PREFIX.GROUP.PROPERTY
			ss := strings.SplitN(k[len(prefix):], ".", 2)
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

// GetProperties 返回所有的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetProperties() map[string]interface{} {
	return p.properties
}

// bindOption 属性值绑定可选项
type bindOption struct {
	propNamePrefix string // 属性名前缀
	fullPropName   string // 完整属性名
	fieldName      string // 结构体字段的名称
}

// bindStruct 对结构体进行属性值绑定
func bindStruct(p Properties, v reflect.Value, opt bindOption) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)

		// 可能会开放私有字段
		fv = SpringUtils.PatchValue(fv, true)
		subFieldName := opt.fieldName + ".$" + ft.Name

		// 字段的绑定可选项
		subOpt := bindOption{
			propNamePrefix: opt.propNamePrefix,
			fullPropName:   opt.fullPropName,
			fieldName:      subFieldName,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			bindStructField(p, fv, tag, subOpt)
			continue
		}

		// 匿名嵌套需要处理，不是结构体的具名字段无需处理
		if ft.Anonymous || ft.Type.Kind() == reflect.Struct {
			bindStruct(p, fv, subOpt)
		}
	}
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

// bindStructField 对结构体的字段进行属性绑定
func bindStructField(p Properties, v reflect.Value, str string, opt bindOption) {

	// 检查 tag 语法是否正确
	if !(strings.HasPrefix(str, "${") && strings.HasSuffix(str, "}")) {
		panic(fmt.Errorf("%s 属性绑定的语法发生错误", opt.fieldName))
	}

	// 指针不能作为属性绑定的目标
	if v.Kind() == reflect.Ptr {
		panic(fmt.Errorf("%s 属性绑定的目标不能是指针", opt.fieldName))
	}

	key, def := parsePropertyTag(str[2 : len(str)-1])

	// 此处使用最短属性名
	if opt.fullPropName == "" {
		opt.fullPropName = key
	} else if key != "" {
		opt.fullPropName = opt.fullPropName + "." + key
	}

	// 属性名如果有前缀要加上前缀
	if opt.propNamePrefix != "" {
		key = opt.propNamePrefix + "." + key
	}

	bindValue(p, v, key, def, opt)
}

// resolveProperty 解析属性值，查看其是否具有引用关系
func resolveProperty(p Properties, _ string, value interface{}) interface{} {
	str, ok := value.(string)

	// 不是字符串或者没有使用配置引用语法
	if !ok || !strings.HasPrefix(str, "${") {
		return value
	}

	key, def := parsePropertyTag(str[2 : len(str)-1])
	if val, _ := p.GetDefaultProperty(key, def); val != nil {
		return resolveProperty(p, key, val)
	}

	panic(fmt.Errorf("property \"%s\" not config", key))
}

func getPropertyValue(p Properties, k reflect.Kind, key string, def interface{}, opt bindOption) interface{} {

	// 首先获取精确匹配的属性值
	if val, ok := p.GetDefaultProperty(key, nil); ok {
		return val
	}

	// Map 和 Struct 类型获取具有相同前缀的属性值
	if k == reflect.Map || k == reflect.Struct {
		if prefixValue := p.GetPrefixProperties(key); len(prefixValue) > 0 {
			return prefixValue
		}
	}

	// 最后使用默认值，需要解析配置引用语法
	if def != nil {
		return resolveProperty(p, key, def)
	}

	panic(fmt.Errorf("%s properties \"%s\" not config", opt.fieldName, opt.fullPropName))
}

// bindValue 对任意 value 进行属性绑定
func bindValue(p Properties, v reflect.Value, key string, def interface{}, opt bindOption) {

	t := v.Type()
	k := t.Kind()

	// 存在值类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := p.TypeConverters()[t]; ok {
		propValue := getPropertyValue(p, k, key, def, opt)
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
		v.Set(out[0])
		return
	}

	if k == reflect.Struct {
		if def == nil {
			bindStruct(p, v, bindOption{
				propNamePrefix: key,
				fullPropName:   opt.fullPropName,
				fieldName:      opt.fieldName,
			})
			return
		} else { // 前面已经校验过是否存在值类型转换器
			panic(fmt.Errorf("%s 结构体字段不能指定默认值", opt.fieldName))
		}
	}

	propValue := getPropertyValue(p, k, key, def, opt)

	switch k {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		if u, err := cast.ToUint64E(propValue); err == nil {
			v.SetUint(u)
		} else {
			panic(fmt.Errorf("property value %s isn't uint type", opt.fullPropName))
		}
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		if i, err := cast.ToInt64E(propValue); err == nil {
			v.SetInt(i)
		} else {
			panic(fmt.Errorf("property value %s isn't int type", opt.fullPropName))
		}
	case reflect.Float64, reflect.Float32:
		if f, err := cast.ToFloat64E(propValue); err == nil {
			v.SetFloat(f)
		} else {
			panic(fmt.Errorf("property value %s isn't float type", opt.fullPropName))
		}
	case reflect.String:
		if s, err := cast.ToStringE(propValue); err == nil {
			v.SetString(s)
		} else {
			panic(fmt.Errorf("property value %s isn't string type", opt.fullPropName))
		}
	case reflect.Bool:
		if b, err := cast.ToBoolE(propValue); err == nil {
			v.SetBool(b)
		} else {
			panic(fmt.Errorf("property value %s isn't bool type", opt.fullPropName))
		}
	case reflect.Slice:
		elemType := v.Type().Elem()
		elemKind := elemType.Kind()

		// 如果是字符串的话，尝试按照逗号进行切割
		if s, ok := propValue.(string); ok {
			propValue = strings.Split(s, ",")
		}

		// 处理使用类型转换器的场景
		if fn, ok := p.TypeConverters()[elemType]; ok {
			if s0, err := cast.ToStringSliceE(propValue); err == nil {
				sv := reflect.MakeSlice(t, len(s0), len(s0))
				fnValue := reflect.ValueOf(fn)
				for i, iv := range s0 {
					res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
					sv.Index(i).Set(res[0])
				}
				v.Set(sv)
				return
			} else {
				panic(fmt.Errorf("property value %s isn't []string type", opt.fullPropName))
			}
		}

		switch elemKind {
		case reflect.Uint64:
			if i, err := ToUint64SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []uint64 type", opt.fullPropName))
			}
		case reflect.Uint32:
			if i, err := ToUint32SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []uint32 type", opt.fullPropName))
			}
		case reflect.Uint16:
			if i, err := ToUint16SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []uint16 type", opt.fullPropName))
			}
		case reflect.Uint8:
			if i, err := ToUint8SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []uint8 type", opt.fullPropName))
			}
		case reflect.Uint:
			if i, err := ToUintSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []uint type", opt.fullPropName))
			}
		case reflect.Int64:
			if i, err := ToInt64SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []int64 type", opt.fullPropName))
			}
		case reflect.Int32:
			if i, err := ToInt32SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []int32 type", opt.fullPropName))
			}
		case reflect.Int16:
			if i, err := ToInt16SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []int16 type", opt.fullPropName))
			}
		case reflect.Int8:
			if i, err := ToInt8SliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []int8 type", opt.fullPropName))
			}
		case reflect.Int:
			if i, err := ToIntSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []int type", opt.fullPropName))
			}
		case reflect.Float64, reflect.Float32:
			panic(errors.New("暂未支持"))
		case reflect.String:
			if i, err := cast.ToStringSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				panic(fmt.Errorf("property value %s isn't []string type", opt.fullPropName))
			}
		case reflect.Bool:
			if b, err := cast.ToBoolSliceE(propValue); err == nil {
				v.Set(reflect.ValueOf(b))
			} else {
				panic(fmt.Errorf("property value %s isn't []bool type", opt.fullPropName))
			}
		default:
			// 处理结构体字段的场景
			if s, ok := propValue.([]interface{}); ok {
				result := reflect.MakeSlice(t, len(s), len(s))
				for i, si := range s {
					if sv, err := cast.ToStringMapE(si); err == nil {
						ev := reflect.New(elemType)
						subFullPropName := fmt.Sprintf("%s[%d]", key, i)
						bindStruct(&defaultProperties{sv, p.TypeConverters()}, ev.Elem(), bindOption{
							fullPropName: subFullPropName,
							fieldName:    opt.fieldName,
						})
						result.Index(i).Set(ev.Elem())
					} else {
						panic(fmt.Errorf("property value %s isn't []map[string]interface{}", opt.fullPropName))
					}
				}
				v.Set(result)
			} else {
				panic(fmt.Errorf("property value %s isn't []map[string]interface{}", opt.fullPropName))
			}
		}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			panic(fmt.Errorf("field: %s isn't map[string]interface{}", opt.fieldName))
		}

		elemType := t.Elem()
		elemKind := elemType.Kind()

		// 首先处理使用类型转换器的场景
		if fn, ok := p.TypeConverters()[elemType]; ok {
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
				return
			} else {
				panic(fmt.Errorf("property value %s isn't map[string]string", opt.fullPropName))
			}
		}

		switch elemKind {
		case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
			panic(errors.New("暂未支持"))
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
			panic(errors.New("暂未支持"))
		case reflect.Float64, reflect.Float32:
			panic(errors.New("暂未支持"))
		case reflect.Bool:
			panic(errors.New("暂未支持"))
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
				panic(fmt.Errorf("property value %s isn't map[string]string", opt.fullPropName))
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
					subFullPropName := fmt.Sprintf("%s.%s", key, k1)
					bindStruct(&defaultProperties{v1, p.TypeConverters()}, ev.Elem(), bindOption{
						fullPropName: subFullPropName,
						fieldName:    opt.fieldName,
					})
					result.SetMapIndex(reflect.ValueOf(k1), ev.Elem())
				}

				v.Set(result)
			} else {
				panic(fmt.Errorf("property value %s isn't map[string]map[string]interface{}", opt.fullPropName))
			}
		}
	default:
		panic(errors.New(opt.fieldName + " unsupported type " + v.Kind().String()))
	}
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) BindProperty(key string, i interface{}) {

	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		panic(errors.New("参数 v 必须是一个指针"))
	}

	t := v.Type().Elem()
	s := t.Name() // 当绑定对象是 map 或者 slice 时，取元素的类型名
	if s == "" && (t.Kind() == reflect.Map || t.Kind() == reflect.Slice) {
		s = t.Elem().Name()
	}

	bindValue(p, v.Elem(), key, nil, bindOption{fieldName: s, fullPropName: key})
}

// TypeConverters 类型转换器集合
func (p *defaultProperties) TypeConverters() map[reflect.Type]TypeConverter {
	return p.converters
}

// validTypeConverter 返回是否是合法的类型转换器，类型转换器要求：
// 必须是函数，且只能有一个字符串类型的输入参数和一个值类型的输出参数。
func validTypeConverter(t reflect.Type) bool {

	// 必须是函数 && 只能有一个输入参数 && 只能有一个输出参数
	if t.Kind() != reflect.Func || t.NumIn() != 1 || t.NumOut() != 1 {
		return false
	}

	inType := t.In(0)
	outType := t.Out(0)

	// 输入参数必须是字符串类型 && 输出参数必须是值类型
	return inType.Kind() == reflect.String && IsValueType(outType.Kind())
}

// AddTypeConverter 添加类型转换器
func (p *defaultProperties) AddTypeConverter(fn TypeConverter) {
	if t := reflect.TypeOf(fn); validTypeConverter(t) {
		p.converters[t.Out(0)] = fn
	} else {
		panic(errors.New("fn must be func(string)type"))
	}
}
