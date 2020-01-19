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
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/spf13/cast"
	"github.com/spf13/viper"
)

func init() {

	// string -> time.Duration 转换器
	RegisterTypeConverter(func(v string) time.Duration {
		return cast.ToDuration(v)
	})

	//  string -> time.Time 转换器
	RegisterTypeConverter(func(v string) time.Time {
		return cast.ToTime(v)
	})
}

// defaultProperties Properties 的默认版本
type defaultProperties struct {
	properties map[string]interface{}
}

// NewDefaultProperties defaultProperties 的构造函数
func NewDefaultProperties() *defaultProperties {
	return &defaultProperties{
		properties: make(map[string]interface{}),
	}
}

// LoadProperties 加载属性配置文件
func (p *defaultProperties) LoadProperties(filename string) {
	SpringLogger.Debug(">>> load properties from", filename)

	v := viper.New()
	v.SetConfigFile(filename)
	if err := v.ReadInConfig(); err != nil {
		SpringLogger.Panic(err)
	}

	keys := v.AllKeys()
	sort.Strings(keys)

	for _, key := range keys {
		val := v.Get(key)
		p.SetProperty(key, val)
		SpringLogger.Debugf("%s=%v", key, val)
	}
}

// GetProperty 返回属性值，属性名称统一转成小写。
func (p *defaultProperties) GetProperty(name string) interface{} {
	name = strings.ToLower(name)
	return p.properties[name]
}

// GetBoolProperty 返回布尔型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetBoolProperty(name string) bool {
	return cast.ToBool(p.GetProperty(name))
}

// GetIntProperty 返回有符号整型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetIntProperty(name string) int64 {
	return cast.ToInt64(p.GetProperty(name))
}

// GetUintProperty 返回无符号整型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetUintProperty(name string) uint64 {
	return cast.ToUint64(p.GetProperty(name))
}

// GetFloatProperty 返回浮点型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetFloatProperty(name string) float64 {
	return cast.ToFloat64(p.GetProperty(name))
}

// GetStringProperty 返回字符串型属性值，属性名称统一转成小写。
func (p *defaultProperties) GetStringProperty(name string) string {
	return cast.ToString(p.GetProperty(name))
}

// SetProperty 设置属性值，属性名称统一转成小写。
func (p *defaultProperties) SetProperty(name string, value interface{}) {
	name = strings.ToLower(name)
	p.properties[name] = value
}

// GetDefaultProperty 返回属性值，如果没有找到则使用指定的默认值
func (p *defaultProperties) GetDefaultProperty(name string, defaultValue interface{}) (interface{}, bool) {
	name = strings.ToLower(name)
	if v, ok := p.properties[name]; ok {
		return v, true
	}
	return defaultValue, false
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

// GetAllProperties 返回所有的属性值，属性名称统一转成小写。
func (p *defaultProperties) GetAllProperties() map[string]interface{} {
	return p.properties
}

// bindStruct 对结构体进行属性值绑定
func bindStruct(prop Properties, beanType reflect.Type, beanValue reflect.Value,
	fieldName string, propNamePrefix string, allAccess bool) {

	// 遍历结构体的所有字段
	for i := 0; i < beanType.NumField(); i++ {
		it := beanType.Field(i)
		iv := beanValue.Field(i)

		// 可能会开放私有字段
		iv = SpringUtils.ValuePatchIf(iv, allAccess)
		subFieldName := fieldName + ".$" + it.Name

		if it.Anonymous { // 处理结构体嵌套的情况
			if _, ok := it.Tag.Lookup("value"); ok {
				SpringLogger.Panic(subFieldName + " 嵌套结构体上不允许有 value 标签")
			}
			bindStruct(prop, it.Type, iv, subFieldName, propNamePrefix, allAccess)
			continue
		}

		if tag, ok := it.Tag.Lookup("value"); ok { // 处理有 value 标签字段
			bindStructField(prop, it.Type, iv, subFieldName, propNamePrefix, tag, allAccess)
		} else {
			if it.Type.Kind() == reflect.Struct { // 处理不带标签的结构体字段
				bindStruct(prop, it.Type, iv, subFieldName, propNamePrefix, allAccess)
			}
		}
	}
}

// bindStructField 对结构体的字段进行属性绑定
func bindStructField(prop Properties, fieldType reflect.Type, fieldValue reflect.Value,
	fieldName string, propNamePrefix string, propTag string, allAccess bool) {

	// 检查语法是否正确
	if !(strings.HasPrefix(propTag, "${") && strings.HasSuffix(propTag, "}")) {
		SpringLogger.Panic(fieldName + " 属性绑定的语法发生错误")
	}

	// 指针不能作为属性绑定的目标
	if fieldValue.Kind() == reflect.Ptr {
		SpringLogger.Panic(fieldName + " 属性绑定的目标不能是指针")
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

	bindValue(prop, fieldType, fieldValue, fieldName, propName, defaultValue, allAccess)
}

// bindValue 对任意 value 进行属性绑定
func bindValue(prop Properties, beanType reflect.Type, beanValue reflect.Value,
	fieldName string, propName string, defaultValue interface{}, allAccess bool) {

	getPropValue := func() interface{} { // 获取最终决定的属性值
		if val, ok := prop.GetDefaultProperty(propName, nil); ok {
			return val
		} else {
			if defaultValue != nil {
				return defaultValue
			}

			// 尝试找一下具有相同前缀的属性值的列表
			if prefixValue := prop.GetPrefixProperties(propName); len(prefixValue) > 0 {
				return prefixValue
			}

			SpringLogger.Panic(fieldName + " properties \"" + propName + "\" not config")
			return nil
		}
	}

	// 存在类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := typeConverters[beanType]; ok {
		propValue := getPropValue()
		fnValue := reflect.ValueOf(fn)
		res := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
		beanValue.Set(res[0])
		return
	}

	if beanValue.Kind() == reflect.Struct {
		if defaultValue != nil { // 结构体字段不能指定默认值
			SpringLogger.Panic(fieldName + " 结构体字段不能指定默认值")
		}
		bindStruct(prop, beanType, beanValue, fieldName, propName, allAccess)
		return
	}

	// 获取属性值
	propValue := getPropValue()

	switch beanValue.Kind() {
	case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
		u := cast.ToUint64(propValue)
		beanValue.SetUint(u)
	case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
		i := cast.ToInt64(propValue)
		beanValue.SetInt(i)
	case reflect.Float64, reflect.Float32:
		f := cast.ToFloat64(propValue)
		beanValue.SetFloat(f)
	case reflect.String:
		s := cast.ToString(propValue)
		beanValue.SetString(s)
	case reflect.Bool:
		b := cast.ToBool(propValue)
		beanValue.SetBool(b)
	case reflect.Slice:
		{
			elemType := beanValue.Type().Elem()
			elemKind := elemType.Kind()

			switch elemKind {
			case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
				SpringLogger.Panic("暂未支持")
			case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
				i := cast.ToIntSlice(propValue)
				beanValue.Set(reflect.ValueOf(i))
			case reflect.Float64, reflect.Float32:
				SpringLogger.Panic("暂未支持")
			case reflect.String:
				i := cast.ToStringSlice(propValue)
				beanValue.Set(reflect.ValueOf(i))
			case reflect.Bool:
				b := cast.ToBoolSlice(propValue)
				beanValue.Set(reflect.ValueOf(b))
			default:
				if fn, ok := typeConverters[elemType]; ok {
					// 首先处理使用类型转换器的场景

					fnValue := reflect.ValueOf(fn)
					s0 := cast.ToStringSlice(propValue)
					sv := reflect.MakeSlice(beanType, len(s0), len(s0))

					for i, iv := range s0 {
						res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
						sv.Index(i).Set(res[0])
					}

					beanValue.Set(sv)

				} else { // 然后处理结构体字段的场景

					if s, isArray := propValue.([]interface{}); isArray {
						result := reflect.MakeSlice(beanType, len(s), len(s))

						for i, si := range s {
							if sv, err := cast.ToStringMapE(si); err == nil {
								ev := reflect.New(elemType)
								bindStruct(&defaultProperties{sv}, elemType, ev.Elem(), fieldName, "", allAccess)
								result.Index(i).Set(ev.Elem())
							} else {
								SpringLogger.Panicf("property %s isn't []map[string]interface{}", propName)
							}
						}

						beanValue.Set(result)

					} else {
						SpringLogger.Panicf("property %s isn't []map[string]interface{}", propName)
					}
				}
			}
		}
	case reflect.Map:
		if beanType.Key().Kind() != reflect.String {
			SpringLogger.Panicf("field: %s isn't map[string]interface{}", fieldName)
		}

		elemType := beanType.Elem()
		elemKind := elemType.Kind()

		switch elemKind {
		case reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint:
			SpringLogger.Panic("暂未支持")
		case reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int:
			SpringLogger.Panic("暂未支持")
		case reflect.Float64, reflect.Float32:
			SpringLogger.Panic("暂未支持")
		case reflect.String:
			if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
				trimKey := propName + "."
				result := make(map[string]string)
				for k0, v0 := range mapValue {
					k0 = strings.TrimPrefix(k0, trimKey)
					result[k0] = v0
				}
				beanValue.Set(reflect.ValueOf(result))
			} else {
				SpringLogger.Panicf("property %s isn't map[string]string", propName)
			}
		case reflect.Bool:
			SpringLogger.Panic("暂未支持")
		default:
			if fn, ok := typeConverters[elemType]; ok {
				// 首先处理使用类型转换器的场景

				if mapValue, err := cast.ToStringMapStringE(propValue); err == nil {
					trimKey := propName + "."
					fnValue := reflect.ValueOf(fn)
					result := reflect.MakeMap(beanType)
					for k0, v0 := range mapValue {
						res := fnValue.Call([]reflect.Value{reflect.ValueOf(v0)})
						k0 = strings.TrimPrefix(k0, trimKey)
						result.SetMapIndex(reflect.ValueOf(k0), res[0])
					}
					beanValue.Set(result)
				} else {
					SpringLogger.Panicf("property %s isn't map[string]string", propName)
				}

			} else { // 然后处理结构体字段的场景

				if mapValue, err := cast.ToStringMapE(propValue); err == nil {

					temp := make(map[string]map[string]interface{})
					trimKey := propName + "."

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

					result := reflect.MakeMapWithSize(beanType, len(temp))
					for k1, v1 := range temp {
						ev := reflect.New(elemType)
						bindStruct(&defaultProperties{v1}, elemType, ev.Elem(), fieldName, "", allAccess)
						result.SetMapIndex(reflect.ValueOf(k1), ev.Elem())
					}

					beanValue.Set(result)

				} else {
					SpringLogger.Panicf("property %s isn't map[string]map[string]interface{}", propName)
				}
			}
		}
	default:
		SpringLogger.Panic(fieldName + " unsupported type " + beanValue.Kind().String())
	}
}

// BindProperty 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) BindProperty(name string, i interface{}) {
	p.BindPropertyIf(name, i, false)
}

// BindPropertyIf 根据类型获取属性值，属性名称统一转成小写。
func (p *defaultProperties) BindPropertyIf(name string, i interface{}, allAccess bool) {
	v := reflect.ValueOf(i)
	if v.Kind() != reflect.Ptr {
		SpringLogger.Panic("参数 v 必须是一个指针")
	}
	t := v.Type().Elem()
	bindValue(p, t, v.Elem(), t.Name(), name, nil, allAccess)
}
