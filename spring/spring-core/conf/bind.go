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

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-boost/util"
)

var (
	ErrNotExist = errors.New("not exist")
)

type BindParam struct {
	Type   reflect.Type // 绑定对象的类型
	Key    string       // 完整的属性名
	Path   string       // 绑定对象的路径
	def    string       // 默认值
	hasDef bool         // 是否具有默认值
}

func (param *BindParam) BindTag(tag string) error {

	if !validTag(tag) {
		return fmt.Errorf("%s 属性绑定字符串 %q 语法错误", param.Path, tag)
	}

	key, def, hasDef := parseTag(tag)
	if param.Key == "" {
		param.Key = key
	} else if key != "" {
		param.Key = param.Key + "." + key
	}

	param.hasDef = hasDef
	param.def = def
	return nil
}

func BindValue(p *Properties, v reflect.Value, param BindParam) error {

	if !util.IsValueType(param.Type) {
		return fmt.Errorf("%s 属性绑定的目标必须是值类型", param.Path)
	}

	log.Tracef("::<>:: %#v", param)

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, param)
	case reflect.Array:
		return bindArray(p, v, param)
	case reflect.Slice:
		return bindSlice(p, v, param)
	}

	fn, _ := converters[param.Type]
	if v.Kind() == reflect.Struct {
		if fn == nil {
			return bindStruct(p, v, param)
		}
	}

	val, err := resolve(p, param)
	if err != nil {
		return fmt.Errorf("type %q bind error: %w", param.Type, err)
	}

	if fn != nil {
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(val)})
		if !out[1].IsNil() {
			return out[1].Interface().(error)
		}
		v.Set(out[0])
		return nil
	}

	switch v.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := cast.ToUint64E(val)
		if err == nil {
			v.SetUint(u)
		}
		return err
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := cast.ToInt64E(val)
		if err == nil {
			v.SetInt(i)
		}
		return err
	case reflect.Float32, reflect.Float64:
		f, err := cast.ToFloat64E(val)
		if err == nil {
			v.SetFloat(f)
		}
		return err
	case reflect.Bool:
		b, err := cast.ToBoolE(val)
		if err == nil {
			v.SetBool(b)
		}
		return err
	case reflect.String:
		s, err := cast.ToStringE(val)
		if err == nil {
			v.SetString(s)
		}
		return err
	}

	return fmt.Errorf("unsupported bind type %q", param.Type.String())
}

func bindArray(p *Properties, v reflect.Value, param BindParam) error {

	if param.hasDef {
		if param.def == "" {
			return nil
		}
		return fmt.Errorf("%s array 类型不能指定非空默认值", param.Path)
	}

	for i := 0; i < v.Len(); i++ {

		subValue := v.Index(i)
		subKey := fmt.Sprintf("%s[%d]", param.Key, i)
		subPath := fmt.Sprintf("%s[%d]", param.Path, i)

		subParam := BindParam{
			Type: subValue.Type(),
			Key:  subKey,
			Path: subPath,
		}

		err := BindValue(p, subValue, subParam)
		if errors.Is(err, ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func bindSlice(p *Properties, v reflect.Value, param BindParam) error {

	if param.hasDef {
		if param.def == "" {
			return nil
		}
		return fmt.Errorf("%s slice 类型不能指定非空默认值", param.Path)
	}

	et := param.Type.Elem()
	slice := reflect.MakeSlice(param.Type, 0, 0)

	for i := 0; ; i++ {

		subKey := fmt.Sprintf("%s[%d]", param.Key, i)
		subPath := fmt.Sprintf("%s[%d]", param.Path, i)
		subParam := BindParam{Type: et, Key: subKey, Path: subPath}

		e := reflect.New(et).Elem()
		err := BindValue(p, e, subParam)
		if errors.Is(err, ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}

		slice = reflect.Append(slice, e)
	}

	v.Set(slice)
	return nil
}

func bindMap(p *Properties, v reflect.Value, param BindParam) error {

	if param.hasDef {
		if param.def == "" {
			return nil
		}
		return fmt.Errorf("%s map 类型不能指定非空默认值", param.Path)
	}

	var keys []string
	{
		var keyPath []string
		if param.Key != "" {
			keyPath = strings.Split(param.Key, ".")
		}
		t := p.t
		for i, s := range keyPath {
			vt, ok := t[s]
			if !ok {
				return fmt.Errorf("property %q %w", param.Key, ErrNotExist)
			}
			if _, ok = vt.(struct{}); ok {
				oldKey := strings.Join(keyPath[:i+1], ".")
				return fmt.Errorf("property %q has a value but want another sub key %q", oldKey, param.Key+".*")
			}
			t = vt.(map[string]interface{})
		}
		for k := range t {
			keys = append(keys, k)
		}
	}

	et := param.Type.Elem()
	m := reflect.MakeMap(param.Type)
	for _, key := range keys {
		e := reflect.New(et).Elem()
		subKey := key
		if param.Key != "" {
			subKey = param.Key + "." + key
		}
		subParam := BindParam{
			Type: et,
			Key:  subKey,
			Path: param.Path,
		}
		err := BindValue(p, e, subParam)
		if err != nil {
			return err
		}
		m.SetMapIndex(reflect.ValueOf(key), e)
	}
	v.Set(m)
	return nil
}

func bindStruct(p *Properties, v reflect.Value, param BindParam) error {

	if param.hasDef && param.def != "" {
		return fmt.Errorf("%s struct 类型不能指定非空默认值", param.Path)
	}

	for i := 0; i < param.Type.NumField(); i++ {
		ft := param.Type.Field(i)
		fv := v.Field(i)

		if !fv.CanInterface() {
			fv = util.PatchValue(fv)
			if !fv.CanInterface() {
				continue
			}
		}

		subParam := BindParam{
			Type: ft.Type,
			Key:  param.Key,
			Path: param.Path + "." + ft.Name,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			if err := subParam.BindTag(tag); err != nil {
				return err
			}
			if err := BindValue(p, fv, subParam); err != nil {
				return err
			}
			continue
		}

		// 指针或者结构体类型可能出现无限递归的情况。
		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := bindStruct(p, fv, subParam); err != nil {
				return err
			}
		}
	}
	return nil
}

// validTag 返回是否为 ${key:=def} 格式的字符串。
func validTag(tag string) bool {
	return strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")
}

// parseTag 解析 ${key:=def} 格式的字符串，然后返回 key 和 def 的值。
func parseTag(tag string) (key string, def string, hasDef bool) {
	ss := strings.SplitN(tag[2:len(tag)-1], ":=", 2)
	if len(ss) > 1 {
		hasDef = true
		def = ss[1]
	}
	key = ss[0]
	return
}

func resolveString(p *Properties, s string) (string, error) {
	n := len(s)

	count := 0
	found := false
	start, end := -1, -1
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '$':
			if i < n-1 {
				if s[i+1] == '{' {
					if count == 0 {
						start = i
					}
					count++
				}
			}
		case '}':
			count--
			if count == 0 {
				found = true
				end = i
			}
		}
		if found {
			break
		}
	}

	if start < 0 || end < 0 {
		return s, nil
	}

	if count > 0 {
		return "", fmt.Errorf("%s 语法错误", s)
	}

	param := BindParam{}
	err := param.BindTag(s[start : end+1])
	if err != nil {
		return "", err
	}

	s1, err := resolve(p, param)
	if err != nil {
		return "", err
	}

	s2, err := resolveString(p, s[end+1:])
	if err != nil {
		return "", err
	}

	return s[:start] + s1 + s2, nil
}

// resolve 解析 ${key:=def} 字符串，返回 key 对应的属性值，如果没有找到则返回
// def 值，如果 def 存在引用则递归解析直到获取最终的属性值。
func resolve(p *Properties, param BindParam) (string, error) {
	val := p.Get(param.Key)
	if val == "" {
		if p.Has(param.Key) {
			return "", nil
		}
		if param.hasDef {
			val = param.def
		} else {
			return "", fmt.Errorf("property %q %w", param.Key, ErrNotExist)
		}
	}
	return resolveString(p, val)
}
