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

	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

var (
	ErrNotExist = errors.New("not exist")
)

type properties interface {
	Get(key string, opts ...GetOption) interface{}
}

type bindOption struct {
	key    string // 完整的属性 key
	path   string // 结构体字段 path
	def    string // 默认值
	hasDef bool   // def 字段是否有效
}

func bind(p properties, v reflect.Value, tag string, opt bindOption) error {

	if v.Kind() == reflect.Ptr {
		return fmt.Errorf("%s 属性绑定的目标不能是指针", opt.path)
	}

	if !validTag(tag) {
		return fmt.Errorf("%s 属性绑定字符串 %s 发生错误", opt.path, tag)
	}

	key, def, hasDef := parseTag(tag)
	if key == "" {
		key = "ANONYMOUS"
	}

	if opt.key == "" {
		opt.key = key
	} else {
		opt.key = opt.key + "." + key
	}

	opt.def = def
	opt.hasDef = hasDef

	return bindValue(p, v, opt)
}

func bindValue(p properties, v reflect.Value, opt bindOption) error {

	log.Tracef("::<>:: %#v", opt)

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, opt)
	case reflect.Array:
		return bindArray(p, v, opt)
	case reflect.Slice:
		return bindSlice(p, v, opt)
	}

	t := v.Type()
	fn, _ := converters[t]

	if v.Kind() == reflect.Struct {
		if fn == nil {
			return bindStruct(p, v, opt)
		}
	}

	val, err := resolve(p, opt)
	if err != nil {
		return err
	}

	if fn != nil {
		fnValue := reflect.ValueOf(fn)
		in := []reflect.Value{reflect.ValueOf(val)}
		out := fnValue.Call(in)
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

	return errors.New("unsupported type " + t.String())
}

func bindArray(p properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef {
		if opt.def == "" {
			return nil
		}
		return fmt.Errorf("%s array 字段不能指定非空默认值", opt.path)
	}

	for i := 0; i < v.Len(); i++ {
		subKey := fmt.Sprintf("%s[%d]", opt.key, i)
		subPath := fmt.Sprintf("%s[%d]", opt.path, i)
		subOpt := bindOption{key: subKey, path: subPath}
		err := bindValue(p, v.Index(i), subOpt)
		if errors.Is(err, ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func bindSlice(p properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef {
		if opt.def == "" {
			return nil
		}
		return fmt.Errorf("%s slice 字段不能指定非空默认值", opt.path)
	}

	t := v.Type()
	et := t.Elem()
	slice := reflect.MakeSlice(t, 0, 0)

	for i := 0; ; i++ {
		subKey := fmt.Sprintf("%s[%d]", opt.key, i)
		subPath := fmt.Sprintf("%s[%d]", opt.path, i)
		subOpt := bindOption{key: subKey, path: subPath}

		e := reflect.New(et).Elem()
		err := bindValue(p, e, subOpt)
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

func bindMap(p properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef {
		if opt.def == "" {
			return nil
		}
		return fmt.Errorf("%s map 字段不能指定非空默认值", opt.path)
	}

	g := p.(interface{ Group(prefix string) []string })
	groups := g.Group(opt.key)

	t := v.Type()
	et := t.Elem()
	m := reflect.MakeMap(t)

	for _, key := range groups {
		e := reflect.New(et).Elem()
		subKey := fmt.Sprintf("%s.%s", opt.key, key)
		subOpt := bindOption{key: subKey, path: opt.path}
		err := bindValue(p, e, subOpt)
		if err != nil {
			return err
		}
		m.SetMapIndex(reflect.ValueOf(key), e)
	}

	v.Set(m)
	return nil
}

func bindStruct(p properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef && opt.def != "" {
		// 空默认值时为什么不直接返回？因为继续处理可以使用结构体字段的默认值。
		return fmt.Errorf("%s struct 字段不能指定非空默认值", opt.path)
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)

		if !fv.CanInterface() {
			fv = util.PatchValue(fv)
			if !fv.CanInterface() {
				continue
			}
		}

		subOpt := bindOption{
			key:  opt.key,
			path: opt.path + "." + ft.Name,
		}

		if tag, ok := ft.Tag.Lookup("value"); ok {
			err := bind(p, fv, tag, subOpt)
			if err != nil {
				return err
			}
			continue
		}

		// 指针或者结构体字段可能出现无限递归的情况。
		if ft.Type.Kind() == reflect.Struct {
			err := bindStruct(p, fv, subOpt)
			if err != nil {
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

func resolveString(p properties, s string) (string, error) {
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

	key, def, hasDef := parseTag(s[start : end+1])
	opt := bindOption{key: key, def: def, hasDef: hasDef}
	s1, err := resolve(p, opt)
	if err != nil {
		return "", err
	}

	s2, err := resolveString(p, s[end+1:])
	if err != nil {
		return "", err
	}

	return s[:start] + s1 + s2, nil
}

func resolve(p properties, opt bindOption) (string, error) {
	val := p.Get(opt.key)
	if val == nil {
		if opt.hasDef {
			val = opt.def
		} else {
			return "", fmt.Errorf("property %q %w", opt.key, ErrNotExist)
		}
	}
	return resolveString(p, val.(string))
}
