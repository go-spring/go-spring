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
	ErrNotExist = errors.New("not exist")
)

// IsPrimitiveValueType 返回是否是原生值类型。首先，什么是值类型？在发生赋值时，如
// 果传递的是数据本身而不是数据的引用，则称这种类型为值类型。那什么是原生值类型？所谓原
// 生值类型是指 golang 定义的 26 种基础类型里面符合值类型定义的类型。罗列下来，就是说
// Bool、Int、Int8、Int16、Int32、Int64、Uint、Uint8、Uint16、Uint32、Uint64、
// Float32、Float64、Complex64、Complex128、String、Struct 这些基础数据类型都
// 是值类型。当然，需要特别说明的是 Struct 类型必须在保证所有字段都是值类型的时候才是
// 值类型，只要有不是值类型的字段就不是值类型。
func IsPrimitiveValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Float32, reflect.Float64:
		return true
	case reflect.String:
		return true
	case reflect.Bool:
		return true
	}
	return false
}

// IsValueType 返回是否是 value 类型。除了原生值类型，它们的集合类型也是值类型，但
// 是仅限于一层复合结构，即 []string、map[string]struct 这种，像 [][]string 则
// 不是值类型，map[string]map[string]string 也不是值类型，因为程序开发过程中，配
// 置项应当越明确越好，而多层次嵌套结构显然会造成信息的不明确，因此不能是值类型。
func IsValueType(t reflect.Type) bool {
	fn := func(t reflect.Type) bool {
		return IsPrimitiveValueType(t) || t.Kind() == reflect.Struct
	}
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return fn(t.Elem())
	default:
		return fn(t)
	}
}

type BindParam struct {
	Type reflect.Type // 绑定对象的类型
	Key  string       // 完整的属性名
	Path string       // 绑定对象的路径
	Tag  ParsedTag    // 解析后的 tag
}

type ParsedTag struct {
	Key    string // 简短属性名
	Def    string // 默认值
	HasDef bool   // 是否具有默认值
	Split  string // 字符串分割器
}

func (param *BindParam) BindTag(tag string) error {
	parsedTag, err := ParseTag(tag)
	if err != nil {
		return err
	}
	param.Tag = parsedTag
	if param.Key == "" {
		param.Key = parsedTag.Key
	} else if parsedTag.Key != "" {
		param.Key = param.Key + "." + parsedTag.Key
	}
	return nil
}

func BindValue(p *Properties, v reflect.Value, param BindParam) error {

	if !IsValueType(param.Type) {
		return util.Errorf(code.FileLine(), "%s 属性绑定的目标必须是值类型", param.Path)
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

	if p.Has(fmt.Sprintf("%s[%d]", param.Key, 0)) {
		return p, nil
	}

	strVal := ""
	primitive := IsPrimitiveValueType(et)

	if p.Has(param.Key) {
		strVal = p.Get(param.Key)
	} else {
		if !param.Tag.HasDef {
			return nil, util.Errorf(code.FileLine(), "property %q %w", param.Key, ErrNotExist)
		}
		if param.Tag.Def == "" {
			return nil, nil
		}
		if !primitive && converters[et] == nil {
			return nil, util.Errorf(code.FileLine(), "%s 不能为非自定义的复杂类型数组指定非空默认值", param.Path)
		}
		strVal = param.Tag.Def
	}

	if strVal == "" {
		return nil, nil
	}

	var (
		err    error
		arrVal []string
	)

	if s := param.Tag.Split; s == "" {
		arrVal = strings.Split(strVal, ",")
	} else if fn := splitters[s]; fn != nil {
		if arrVal, err = fn(strVal); err != nil {
			return nil, err
		}
	}

	p = New()
	for i, s := range arrVal {
		k := fmt.Sprintf("%s[%d]", param.Key, i)
		if err = p.Set(k, s); err != nil {
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

	if param.Tag.HasDef {
		if param.Tag.Def == "" {
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
		t := p.tree
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

	if param.Tag.HasDef && param.Tag.Def != "" {
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

		if IsValueType(ft.Type) {
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

// ParseTag 解析 ${key:=def}|split 格式的字符串，然后返回 key 和 def 的值。
func ParseTag(tag string) (ret ParsedTag, err error) {
	i := strings.LastIndex(tag, "|")
	if i == 0 {
		err = util.Errorf(code.FileLine(), "%q 语法错误", tag)
		return
	}
	j := strings.LastIndex(tag, "}")
	if j <= 0 {
		err = util.Errorf(code.FileLine(), "%q 语法错误", tag)
		return
	}
	var (
		left  = tag[:j]
		right string
	)
	if i > j {
		right = tag[i+1:]
	}
	k := strings.Index(left, "${")
	if k < 0 {
		err = util.Errorf(code.FileLine(), "%q 语法错误", tag)
		return
	}
	left = left[k+2:]
	ret.Split = right
	ssLeft := strings.SplitN(left, ":=", 2)
	ret.Key = ssLeft[0]
	if len(ssLeft) > 1 {
		ret.HasDef = true
		ret.Def = ssLeft[1]
	}
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
	if val, ok := p.data[param.Key]; ok {
		return resolveString(p, val)
	}
	if param.Tag.HasDef {
		return resolveString(p, param.Tag.Def)
	}
	return "", util.Errorf(code.FileLine(), "property %q %w", param.Key, ErrNotExist)
}
