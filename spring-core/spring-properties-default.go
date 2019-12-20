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
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

func init() {

	// string -> time.Duration
	RegisterTypeConverter(func(v string) time.Duration {
		return cast.ToDuration(v)
	})

	//  string -> time.Time
	RegisterTypeConverter(func(v string) time.Time {
		return cast.ToTime(v)
	})
}

//
// 定义 Properties 的默认版本
//
type DefaultProperties struct {
	properties map[string]interface{}
}

//
// 工厂函数
//
func NewDefaultProperties() *DefaultProperties {
	return &DefaultProperties{
		properties: make(map[string]interface{}),
	}
}

//
// 加载属性配置文件
//
func (p *DefaultProperties) LoadProperties(filename string) {

	if _, err := os.Stat(filename); err != nil {
		return // 这里不需要警告
	}

	fmt.Println(">>> load properties from", filename)

	v := viper.New()
	v.SetConfigFile(filename)
	if err := v.ReadInConfig(); err != nil {
		panic(err)
	}

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.SetProperty(key, val)
		fmt.Printf("%s=%v\n", key, val)
	}
}

//
// 获取属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetProperty(name string) interface{} {
	name = strings.ToLower(name)
	return p.properties[name]
}

//
// 获取布尔型属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetBoolProperty(name string) bool {
	return cast.ToBool(p.GetProperty(name))
}

//
// 获取有符号整型属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetIntProperty(name string) int64 {
	return cast.ToInt64(p.GetProperty(name))
}

//
// 获取无符号整型属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetUintProperty(name string) uint64 {
	return cast.ToUint64(p.GetProperty(name))
}

//
// 获取浮点型属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetFloatProperty(name string) float64 {
	return cast.ToFloat64(p.GetProperty(name))
}

//
// 获取字符串型属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetStringProperty(name string) string {
	return cast.ToString(p.GetProperty(name))
}

//
// 设置属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) SetProperty(name string, value interface{}) {
	name = strings.ToLower(name)
	p.properties[name] = value
}

//
// 获取属性值，如果没有找到则使用指定的默认值
//
func (p *DefaultProperties) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	name = strings.ToLower(name)
	if v, ok := p.properties[name]; ok {
		return v, true
	}
	return defaultValue, false
}

//
// 获取指定前缀的属性值集合，属性名称统一转成小写。
//
func (p *DefaultProperties) GetPrefixProperties(prefix string) map[string]interface{} {
	prefix = strings.ToLower(prefix)
	result := make(map[string]interface{})
	for k, v := range p.properties {
		if strings.HasPrefix(k, prefix) {
			result[k] = v
		}
	}
	return result
}

//
// 获取所有的属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) GetAllProperties() map[string]interface{} {
	return p.properties
}

type PropertyHolder interface {
	GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool)
}

type MapPropertyHolder struct {
	m map[interface{}]interface{}
}

func NewMapPropertyHolder(m map[interface{}]interface{}) *MapPropertyHolder {
	return &MapPropertyHolder{
		m: m,
	}
}

func (p *MapPropertyHolder) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	v, ok := p.m[name]
	return v, ok
}

//
// 对结构体的字段进行属性绑定
//
func bindStructField(prop PropertyHolder, fieldType reflect.Type, fieldValue reflect.Value,
	fieldName string, propNamePrefix string, propTag string) {

	// 检查语法是否正确
	if !(strings.HasPrefix(propTag, "${") && strings.HasSuffix(propTag, "}")) {
		panic(fieldName + " 属性绑定的语法发生错误")
	}

	// 指针不能作为属性绑定的目标
	if fieldValue.Kind() == reflect.Ptr {
		panic(fieldName + " 属性绑定的目标不能是指针")
	}

	ss := strings.Split(propTag[2:len(propTag)-1], ":=")

	var (
		propName     string
		defaultValue interface{}
	)

	propName = ss[0]

	// 属性名如果有前缀要加上前缀
	if propNamePrefix != "" {
		propName = propNamePrefix + "." + propName
	}

	if len(ss) > 1 {
		defaultValue = ss[1]
	}

	bindValue(prop, fieldType, fieldValue, fieldName, propName, defaultValue)
}

//
// 对结构体进行属性值绑定
//
func bindStruct(prop PropertyHolder, t reflect.Type, v reflect.Value,
	fieldName string, propNamePrefix string) {

	for i := 0; i < t.NumField(); i++ {
		it := t.Field(i)
		iv := v.Field(i)

		subFieldName := fieldName + ".$" + it.Name

		if it.Anonymous { // 处理结构体嵌套的情况
			if _, ok := it.Tag.Lookup("value"); ok {
				panic(subFieldName + " 嵌套结构体上不允许有 value 标签")
			}
			bindStruct(prop, it.Type, iv, subFieldName, propNamePrefix)
			continue
		}

		if tag, ok := it.Tag.Lookup("value"); ok {
			bindStructField(prop, it.Type, iv, subFieldName, propNamePrefix, tag)
		} else {
			if it.Type.Kind() == reflect.Struct {
				bindStruct(prop, it.Type, iv, subFieldName, propNamePrefix)
			}
		}
	}
}

//
// 对任意 value 进行属性绑定
//
func bindValue(prop PropertyHolder, fieldType reflect.Type, fieldValue reflect.Value,
	fieldName string, propName string, defaultValue interface{}) {

	getPropValue := func() interface{} { // 获取属性值的局部函数
		if val, ok := prop.GetDefaultProperty(propName, nil); ok {
			return val
		} else {
			if defaultValue != nil {
				return defaultValue
			}
			panic(fieldName + " properties \"" + propName + "\" not config")
		}
	}

	// 存在类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := typeConverters[fieldType]; ok {
		propValue := getPropValue()
		fnValue := reflect.ValueOf(fn)
		res := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
		fieldValue.Set(res[0])
		return
	}

	if fieldValue.Kind() == reflect.Struct {

		if defaultValue != nil {
			panic(fieldName + " 嵌套的结构体属性不能指定默认值")
		}

		// 然后才考虑结构体嵌套的属性绑定
		bindStruct(prop, fieldType, fieldValue, fieldName, propName)
		return
	}

	// 获取属性值
	propValue := getPropValue()

	switch fieldValue.Kind() {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		u := cast.ToUint64(propValue)
		fieldValue.SetUint(u)
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		i := cast.ToInt64(propValue)
		fieldValue.SetInt(i)
	case reflect.Float64, reflect.Float32:
		f := cast.ToFloat64(propValue)
		fieldValue.SetFloat(f)
	case reflect.String:
		s := cast.ToString(propValue)
		fieldValue.SetString(s)
	case reflect.Bool:
		b := cast.ToBool(propValue)
		fieldValue.SetBool(b)
	case reflect.Slice:
		{
			elemType := fieldValue.Type().Elem()
			elemKind := elemType.Kind()

			switch elemKind {
			case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
				panic("暂未支持")
			case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
				i := cast.ToIntSlice(propValue)
				fieldValue.Set(reflect.ValueOf(i))
			case reflect.Float64, reflect.Float32:
				panic("暂未支持")
			case reflect.String:
				i := cast.ToStringSlice(propValue)
				fieldValue.Set(reflect.ValueOf(i))
			case reflect.Bool:
				b := cast.ToBoolSlice(propValue)
				fieldValue.Set(reflect.ValueOf(b))
			default:
				if fn, ok := typeConverters[elemType]; ok {
					// 首先处理使用类型转换器的场景

					fnValue := reflect.ValueOf(fn)
					s0 := cast.ToStringSlice(propValue)
					sv := reflect.MakeSlice(fieldType, len(s0), len(s0))

					for i, iv := range s0 {
						res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
						sv.Index(i).Set(res[0])
					}

					fieldValue.Set(sv)

				} else { // 然后处理结构体嵌套的场景

					if s, isArray := propValue.([]interface{}); isArray {
						result := reflect.MakeSlice(fieldType, len(s), len(s))

						for i, si := range s {
							if sv, isMap := si.(map[interface{}]interface{}); isMap {
								ev := reflect.New(elemType)
								bindStruct(NewMapPropertyHolder(sv), elemType, ev.Elem(), fieldName, "")
								result.Index(i).Set(ev.Elem())
							} else {
								panic(fmt.Sprintf("property %s isn't []map[string]interface{}", propName))
							}
						}

						fieldValue.Set(result)

					} else {
						panic(fmt.Sprintf("property %s isn't []map[string]interface{}", propName))
					}
				}
			}
		}
	default:
		panic(fieldName + " unsupported type " + fieldValue.Kind().String())
	}
}

//
// 根据类型获取属性值，属性名称统一转成小写。
//
func (p *DefaultProperties) BindProperty(name string, i interface{}) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		panic("参数 v 必须是一个指针")
	}
	t := v.Type().Elem()
	bindValue(p, t, v.Elem(), t.Name(), name, nil)
}
