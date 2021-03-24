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
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// ParseValueTag 解析 ${key:=def} 字符串，返回 key 和 def 的值。
func ParseValueTag(tag string) (key string, def interface{}) {
	ss := strings.SplitN(tag, ":=", 2)
	if len(ss) > 1 {
		def = ss[1]
	}
	key = ss[0]
	return
}

// ResolveProperty 解析 ${key:=def} 字符串，返回 key 的属性值，如果没有找到则返回 def 值，
// 如果 def 存在引用关系即 def 的内容是 ${key:=value} 形式，则递归解析直到获取最终的属性值。
func ResolveProperty(p Properties, tagOrValue interface{}) (interface{}, error) {

	// 不是字符串或者没有使用配置引用语法则直接返回
	tag, ok := tagOrValue.(string)
	if !ok || !strings.HasPrefix(tag, "${") {
		return tagOrValue, nil
	}

	key, def := ParseValueTag(tag[2 : len(tag)-1])
	if val := p.Default(key, def); val != nil {
		return ResolveProperty(p, val)
	}

	return nil, fmt.Errorf("property \"%s\" not config", key)
}

type BindOption struct {
	Prefix string // 属性名前缀
	Key    string // 最短属性名
	Path   string // 结构体字段
}

// bindStruct 对结构体的字段进行属性绑定，该方法要求 v 的类型必须是结构体。
func bindStruct(p Properties, v reflect.Value, opt BindOption) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := util.PatchValue(v.Field(i))

		subOpt := BindOption{
			Prefix: opt.Prefix,
			Key:    opt.Key,
			Path:   opt.Path + ".$" + ft.Name,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			err := BindValue(p, fv, tag, subOpt)
			if err != nil {
				return err
			}
			continue
		}

		// 匿名嵌套需要处理，不是结构体的具名字段无需处理
		if ft.Anonymous || ft.Type.Kind() == reflect.Struct {
			err := bindStruct(p, fv, subOpt)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// BindValue 对任意值 v 进行属性绑定，tag 是形如 ${key:=def} 的字符串，v 必须是值类型。
func BindValue(p Properties, v reflect.Value, tag string, opt BindOption) error {

	if !(strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")) {
		return fmt.Errorf("%s 属性绑定的语法 %s 发生错误", opt.Path, tag)
	}

	if v.Kind() == reflect.Ptr {
		return fmt.Errorf("%s 属性绑定的目标不能是指针", opt.Path)
	}

	key, def := ParseValueTag(tag[2 : len(tag)-1])

	// 最短属性名
	if opt.Key == "" {
		opt.Key = key
	} else if key != "" {
		opt.Key = opt.Key + "." + key
	}

	// 完整属性名
	if opt.Prefix != "" {
		key = opt.Prefix + "." + key
	}

	t := v.Type()
	k := t.Kind()

	getProperty := func() (interface{}, error) {

		// 首先获取精确匹配的属性值
		if val := p.Get(key); val != nil {
			return val, nil
		}

		// Map 和 Struct 类型获取具有相同前缀的属性值
		if k == reflect.Map || k == reflect.Struct {
			if prop := p.Prefix(key); len(prop) > 0 {
				return prop, nil
			}
		}

		// 最后使用默认值，需要解析配置引用语法
		if def != nil {
			return ResolveProperty(p, def)
		}

		return nil, fmt.Errorf("%s property \"%s\" not config", opt.Path, opt.Key)
	}

	// 存在值类型转换器的情况下结构体优先使用转换器
	if fn, ok := converters[t]; ok {
		prop, err := getProperty()
		if err != nil {
			return err
		}
		fnValue := reflect.ValueOf(fn) // TODO 处理 error 返回值
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(prop)})
		v.Set(out[0])
		return nil
	}

	if k == reflect.Struct {
		if def != nil {
			return fmt.Errorf("%s 结构体字段不能指定默认值", opt.Path)
		}
		return bindStruct(p, v, BindOption{Prefix: key, Key: opt.Key, Path: opt.Path})
	}

	if converter, ok := kConverters[k]; ok {
		prop, err := getProperty()
		if err != nil {
			return err
		}
		return converter(&v, key, prop, opt)
	}
	return fmt.Errorf("%s unsupported type %s", opt.Path, v.Kind().String())
}

func init() {
	// TODO 诚招牛人把这些类型转换的过程简化和统一 !!!

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {
		if u, err := cast.ToUint64E(prop); err == nil {
			v.SetUint(u)
			return nil
		} else {
			return fmt.Errorf("property value %s isn't uint type", opt.Key)
		}
	}, reflect.Uint64, reflect.Uint32, reflect.Uint16, reflect.Uint8, reflect.Uint)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {
		if i, err := cast.ToInt64E(prop); err == nil {
			v.SetInt(i)
			return nil
		} else {
			return fmt.Errorf("property value %s isn't int type", opt.Key)
		}
	}, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8, reflect.Int)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {
		if f, err := cast.ToFloat64E(prop); err == nil {
			v.SetFloat(f)
			return nil
		} else {
			return fmt.Errorf("property value %s isn't float type", opt.Key)
		}
	}, reflect.Float64, reflect.Float32)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {
		if s, err := cast.ToStringE(prop); err == nil {
			v.SetString(s)
			return nil
		} else {
			return fmt.Errorf("property value %s isn't string type", opt.Key)
		}
	}, reflect.String)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {
		if b, err := cast.ToBoolE(prop); err == nil {
			v.SetBool(b)
			return nil
		} else {
			return fmt.Errorf("property value %s isn't bool type", opt.Key)
		}
	}, reflect.Bool)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {

		t := v.Type()
		elemType := t.Elem()
		elemKind := elemType.Kind()

		// 如果是字符串的话，尝试按照逗号进行切割
		if s, ok := prop.(string); ok {
			prop = strings.Split(s, ",")
		}

		// 处理使用类型转换器的场景
		if fn, ok := converters[elemType]; ok {
			if s0, err := cast.ToStringSliceE(prop); err == nil {
				sv := reflect.MakeSlice(t, len(s0), len(s0))
				fnValue := reflect.ValueOf(fn)
				for i, iv := range s0 {
					res := fnValue.Call([]reflect.Value{reflect.ValueOf(iv)})
					sv.Index(i).Set(res[0])
				}
				v.Set(sv)
				return nil
			} else {
				return fmt.Errorf("property value %s isn't []string type", opt.Key)
			}
		}

		switch elemKind {
		case reflect.Uint64:
			if i, err := ToUint64SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint64 type", opt.Key)
			}
		case reflect.Uint32:
			if i, err := ToUint32SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint32 type", opt.Key)
			}
		case reflect.Uint16:
			if i, err := ToUint16SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint16 type", opt.Key)
			}
		case reflect.Uint8:
			if i, err := ToUint8SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint8 type", opt.Key)
			}
		case reflect.Uint:
			if i, err := ToUintSliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []uint type", opt.Key)
			}
		case reflect.Int64:
			if i, err := ToInt64SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int64 type", opt.Key)
			}
		case reflect.Int32:
			if i, err := ToInt32SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int32 type", opt.Key)
			}
		case reflect.Int16:
			if i, err := ToInt16SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int16 type", opt.Key)
			}
		case reflect.Int8:
			if i, err := ToInt8SliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int8 type", opt.Key)
			}
		case reflect.Int:
			if i, err := ToIntSliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []int type", opt.Key)
			}
		case reflect.Float64, reflect.Float32:
			return errors.New("暂未支持")
		case reflect.String:
			if i, err := cast.ToStringSliceE(prop); err == nil {
				v.Set(reflect.ValueOf(i))
			} else {
				return fmt.Errorf("property value %s isn't []string type", opt.Key)
			}
		case reflect.Bool:
			if b, err := cast.ToBoolSliceE(prop); err == nil {
				v.Set(reflect.ValueOf(b))
			} else {
				return fmt.Errorf("property value %s isn't []bool type", opt.Key)
			}
		default:
			// 处理结构体字段的场景
			if s, ok := prop.([]interface{}); ok {
				result := reflect.MakeSlice(t, len(s), len(s))
				for i, si := range s {
					if sv, err := cast.ToStringMapE(si); err == nil {
						ev := reflect.New(elemType)
						subKey := fmt.Sprintf("%s[%d]", key, i)
						err = bindStruct(Map(sv), ev.Elem(), BindOption{Key: subKey, Path: opt.Path})
						if err != nil {
							return err
						}
						result.Index(i).Set(ev.Elem())
					} else {
						return fmt.Errorf("property value %s isn't []map[string]interface{}", opt.Key)
					}
				}
				v.Set(result)
			} else {
				return fmt.Errorf("property value %s isn't []map[string]interface{}", opt.Key)
			}
		}
		return nil
	}, reflect.Slice)

	kindConvert(func(v *reflect.Value, key string, prop interface{}, opt BindOption) error {

		t := v.Type()
		if t.Key().Kind() != reflect.String {
			return fmt.Errorf("path: %s isn't map[string]interface{}", opt.Path)
		}

		elemType := t.Elem()
		elemKind := elemType.Kind()

		// 首先处理使用类型转换器的场景
		if fn, ok := converters[elemType]; ok {
			if mapValue, err := cast.ToStringMapStringE(prop); err == nil {
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
				return fmt.Errorf("property value %s isn't map[string]string", opt.Key)
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
			if mapValue, err := cast.ToStringMapStringE(prop); err == nil {
				prefix := key + "."
				result := make(map[string]string)
				for k0, v0 := range mapValue {
					k0 = strings.TrimPrefix(k0, prefix)
					result[k0] = v0
				}
				v.Set(reflect.ValueOf(result))
			} else {
				return fmt.Errorf("property value %s isn't map[string]string", opt.Key)
			}
		default:
			// 处理结构体字段的场景
			if mapValue, err := cast.ToStringMapE(prop); err == nil {
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
					subKey := fmt.Sprintf("%s.%s", key, k1)
					err = bindStruct(Map(v1), ev.Elem(), BindOption{Key: subKey, Path: opt.Path})
					if err != nil {
						return err
					}
					result.SetMapIndex(reflect.ValueOf(k1), ev.Elem())
				}

				v.Set(result)
			} else {
				return fmt.Errorf("property value %s isn't map[string]map[string]interface{}", opt.Key)
			}
		}
		return nil
	}, reflect.Map)
}

var kConverters = map[reflect.Kind]kConverter{}

func kindConvert(fn kConverter, kinds ...reflect.Kind) {
	for _, kind := range kinds {
		kConverters[kind] = fn
	}
}

type kConverter func(v *reflect.Value, key string, prop interface{}, opt BindOption) error
