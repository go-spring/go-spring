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
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/log"
	"github.com/go-spring/spring-base/util"
)

var (
	logger = log.GetRootLogger()
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
		return util.Errorf(code.FileLine(), "%s 属性绑定字符串 %q 语法错误", param.Path, tag)
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
		return util.Errorf(code.FileLine(), "%s 属性绑定的目标必须是值类型", param.Path)
	}

	logger.Tracef("::<>:: %#v", param)

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, param)
	case reflect.Array:
		return bindArray(p, v, param)
	case reflect.Slice:
		return bindSlice(p, v, param)
	}

	fn := converters[param.Type]
	if v.Kind() == reflect.Struct {
		if fn == nil {
			return bindStruct(p, v, param)
		}
	}

	val, err := resolve(p, param)
	if err != nil {
		return util.Wrapf(err, code.FileLine(), "type %q bind error", param.Type)
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
		var u uint64
		if u, err = strconv.ParseUint(val, 0, 0); err == nil {
			v.SetUint(u)
			return nil
		}
		return util.Errorf(code.FileLine(), "%+v %w", param, err)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int64
		if i, err = strconv.ParseInt(val, 0, 0); err == nil {
			v.SetInt(i)
			return nil
		}
		return util.Errorf(code.FileLine(), "%+v %w", param, err)
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(val, 64); err == nil {
			v.SetFloat(f)
			return nil
		}
		return util.Errorf(code.FileLine(), "%+v %w", param, err)
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(val); err == nil {
			v.SetBool(b)
			return nil
		}
		return util.Errorf(code.FileLine(), "%+v %w", param, err)
	case reflect.String:
		v.SetString(val)
		return nil
	}

	return util.Errorf(code.FileLine(), "unsupported bind type %q", param.Type.String())
}

func getSliceValue(p *Properties, et reflect.Type, param BindParam) (*Properties, error) {

	strVal := ""
	wantDef := false
	primitive := util.IsPrimitiveValueType(et)

	if primitive {
		if !p.Has(param.Key) {
			wantDef = true
		} else {
			strVal = p.Get(param.Key)
		}
	} else {
		if !p.Has(fmt.Sprintf("%s[%d]", param.Key, 0)) {
			wantDef = true
		} else {
			return p, nil
		}
	}

	if wantDef {
		if !param.hasDef {
			return nil, ErrNotExist
		}
		if param.def == "" {
			return nil, nil
		}
		if !primitive {
			return nil, util.Errorf(code.FileLine(), "%s array 类型不能为简单类型指定非空默认值", param.Path)
		}
		strVal = param.def
	}

	if strVal == "" {
		return nil, nil
	}

	p = New()
	for i, s := range strings.Split(strVal, ",") {
		k := fmt.Sprintf("%s[%d]", param.Key, i)
		if err := p.Set(k, s); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func bindArray(p *Properties, v reflect.Value, param BindParam) error {

	et := param.Type.Elem()
	p, err := getSliceValue(p, et, param)
	if p == nil || err != nil {
		return err
	}

	for i := 0; i < v.Len(); i++ {
		subParam := BindParam{
			Type: et,
			Key:  fmt.Sprintf("%s[%d]", param.Key, i),
			Path: fmt.Sprintf("%s[%d]", param.Path, i),
		}
		err = BindValue(p, v.Index(i), subParam)
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

	et := param.Type.Elem()
	p, err := getSliceValue(p, et, param)
	if p == nil || err != nil {
		return err
	}

	slice := reflect.MakeSlice(param.Type, 0, 0)
	for i := 0; ; i++ {
		subParam := BindParam{
			Type: et,
			Key:  fmt.Sprintf("%s[%d]", param.Key, i),
			Path: fmt.Sprintf("%s[%d]", param.Path, i),
		}
		e := reflect.New(et).Elem()
		err = BindValue(p, e, subParam)
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
		return util.Errorf(code.FileLine(), "%s map 类型不能指定非空默认值", param.Path)
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
				return util.Errorf(code.FileLine(), "property %q %w", param.Key, ErrNotExist)
			}
			if _, ok = vt.(struct{}); ok {
				oldKey := strings.Join(keyPath[:i+1], ".")
				return util.Errorf(code.FileLine(), "property %q has a value but want another sub key %q", oldKey, param.Key+".*")
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
		return util.Errorf(code.FileLine(), "%s struct 类型不能指定非空默认值", param.Path)
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

		if ft.Anonymous {
			// 指针或者结构体类型可能出现无限递归的情况。
			if ft.Type.Kind() != reflect.Struct {
				continue
			}
			if err := bindStruct(p, fv, subParam); err != nil {
				return err
			}
			continue
		}

		if util.IsValueType(ft.Type) {
			if subParam.Key == "" {
				subParam.Key = ft.Name
			} else {
				subParam.Key = subParam.Key + "." + ft.Name
			}
			if err := BindValue(p, fv, subParam); err != nil {
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
		return "", util.Errorf(code.FileLine(), "%s 语法错误", s)
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
	if val, ok := p.m[param.Key]; ok {
		return resolveString(p, val)
	}
	if param.hasDef {
		return resolveString(p, param.def)
	}
	return "", util.Errorf(code.FileLine(), "property %q %w", param.Key, ErrNotExist)
}
