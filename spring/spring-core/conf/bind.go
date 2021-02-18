package conf

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// ParseTag
func ParseTag(str string) (key string, def interface{}) {
	ss := strings.SplitN(str, ":=", 2)
	if len(ss) > 1 {
		def = ss[1]
	}
	key = ss[0]
	return
}

// ResolveProperty 解析属性值，查看其是否具有引用关系
func ResolveProperty(p Properties, value interface{}) (interface{}, error) {
	str, ok := value.(string)

	// 不是字符串或者没有使用配置引用语法
	if !ok || !strings.HasPrefix(str, "${") {
		return value, nil
	}

	key, def := ParseTag(str[2 : len(str)-1])
	if val := p.GetDefault(key, def); val != nil {
		return ResolveProperty(p, val)
	}

	return nil, fmt.Errorf("property \"%s\" not config", key)
}

// BindOption 属性值绑定可选项
type BindOption struct {
	PrefixName string // 属性名前缀
	FullName   string // 完整属性名
	FieldName  string // 结构体字段的名称
}

// TODO 我知道这一大段看着很累，等我有时间了再来优化，我也很头痛 ><
func bindStruct(p Properties, v reflect.Value, opt BindOption) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)

		// 可能会开放私有字段
		fv = util.PatchValue(fv, true)
		subFieldName := opt.FieldName + ".$" + ft.Name

		// 字段的绑定可选项
		subOpt := BindOption{
			PrefixName: opt.PrefixName,
			FullName:   opt.FullName,
			FieldName:  subFieldName,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			if err := BindValue(p, fv, tag, subOpt); err != nil {
				return err
			}
			continue
		}

		// 匿名嵌套需要处理，不是结构体的具名字段无需处理
		if ft.Anonymous || ft.Type.Kind() == reflect.Struct {
			if err := bindStruct(p, fv, subOpt); err != nil {
				return err
			}
		}
	}
	return nil
}

// BindValue 对结构体的字段进行属性绑定
func BindValue(p Properties, v reflect.Value, tag string, opt BindOption) error {

	// 检查 tag 语法是否正确
	if !(strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")) {
		return fmt.Errorf("%s 属性绑定的语法发生错误", opt.FieldName)
	}

	// 指针不能作为属性绑定的目标
	if v.Kind() == reflect.Ptr {
		return fmt.Errorf("%s 属性绑定的目标不能是指针", opt.FieldName)
	}

	key, def := ParseTag(tag[2 : len(tag)-1])

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

	t := v.Type()
	k := t.Kind()

	getProperty := func() (interface{}, error) {

		// 首先获取精确匹配的属性值
		if val := p.Get(key); val != nil {
			return val, nil
		}

		// Map 和 Struct 类型获取具有相同前缀的属性值
		if k == reflect.Map || k == reflect.Struct {
			if prefixValue := p.Prefix(key); len(prefixValue) > 0 {
				return prefixValue, nil
			}
		}

		// 最后使用默认值，需要解析配置引用语法
		if def != nil {
			return ResolveProperty(p, def)
		}

		return nil, fmt.Errorf("%s properties \"%s\" not config", opt.FieldName, opt.FullName)
	}

	// 存在值类型转换器的情况下结构体优先使用属性值绑定
	if fn, ok := converters[t]; ok {
		propValue, err := getProperty()
		if err == nil {
			fnValue := reflect.ValueOf(fn)
			out := fnValue.Call([]reflect.Value{reflect.ValueOf(propValue)})
			v.Set(out[0])
		}
		return err
	}

	if k == reflect.Struct {
		if def == nil {
			return bindStruct(p, v, BindOption{
				PrefixName: key,
				FullName:   opt.FullName,
				FieldName:  opt.FieldName,
			})
		} else { // 前面已经校验过是否存在值类型转换器
			return fmt.Errorf("%s 结构体字段不能指定默认值", opt.FieldName)
		}
	}

	propValue, err := getProperty()
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
		if fn, ok := converters[elemType]; ok {
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
						err = bindStruct(Map(sv), ev.Elem(), BindOption{
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
		if fn, ok := converters[elemType]; ok {
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
					err = bindStruct(Map(v1), ev.Elem(), BindOption{
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
