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
	"time"

	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

type bindOption struct {
	key    string // 完整属性名
	path   string // 结构体字段
	def    string
	hasDef bool
}

func bind(p Properties, v reflect.Value, tag string, opt bindOption) error {

	if v.Kind() == reflect.Ptr {
		return fmt.Errorf("%s 属性绑定的目标不能是指针", opt.path)
	}

	if !validTag(tag) {
		return fmt.Errorf("%s 属性绑定的语法 %s 发生错误", opt.path, tag)
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

var ErrNotExist = errors.New("not exist")

func bindValue(p Properties, v reflect.Value, opt bindOption) error {

	log.Tracef("::<>:: %#v", opt)

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, opt)
	case reflect.Array:
		return bindArray(p, v, opt)
	case reflect.Slice:
		return bindSlice(p, v, opt)
	case reflect.Struct:
		return bindStruct(p, v, opt)
	}

	val, err := resolve(p, opt)
	if err != nil {
		return err
	}

	switch v.Interface().(type) {
	case uint, uint8, uint16, uint32, uint64:
		u, err := cast.ToUint64E(val)
		if err == nil {
			v.SetUint(u)
		}
		return err
	case int, int8, int16, int32, int64:
		i, err := cast.ToInt64E(val)
		if err == nil {
			v.SetInt(i)
		}
		return err
	case float32, float64:
		f, err := cast.ToFloat64E(val)
		if err == nil {
			v.SetFloat(f)
		}
		return err
	case bool:
		b, err := cast.ToBoolE(val)
		if err == nil {
			v.SetBool(b)
		}
		return err
	case string:
		s, err := cast.ToStringE(val)
		if err == nil {
			v.SetString(s)
		}
		return err
	case time.Time:
		t, err := cast.ToTimeE(val)
		if err == nil {
			v.Set(reflect.ValueOf(t))
		}
		return err
	case time.Duration:
		d, err := cast.ToDurationE(val)
		if err == nil {
			v.Set(reflect.ValueOf(d))
		}
		return err
	}

	return nil
}

func bindArray(p Properties, v reflect.Value, opt bindOption) error {

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

func bindSlice(p Properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef {
		if opt.def == "" {
			return nil
		}
		return fmt.Errorf("%s slice 字段不能指定非空默认值", opt.path)
	}

	t := v.Type()
	elemType := t.Elem()
	ret := reflect.MakeSlice(v.Type(), 0, 0)

	for i := 0; ; i++ {
		subKey := fmt.Sprintf("%s[%d]", opt.key, i)
		subPath := fmt.Sprintf("%s[%d]", opt.path, i)
		subOpt := bindOption{key: subKey, path: subPath}

		e := reflect.New(elemType).Elem()
		err := bindValue(p, e, subOpt)
		if errors.Is(err, ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
		ret = reflect.Append(ret, e)
	}

	v.Set(ret)
	return nil
}

func bindMap(p Properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef {
		if opt.def == "" {
			return nil
		}
		return fmt.Errorf("%s map 字段不能指定非空默认值", opt.path)
	}

	t := v.Type()
	elemType := t.Elem()
	ret := reflect.MakeMap(t)

	for _, key := range GroupKeys(p, opt.key) {
		e := reflect.New(elemType).Elem()
		subKey := fmt.Sprintf("%s.%s", opt.key, key)
		subOpt := bindOption{key: subKey, path: opt.path}
		if err := bindValue(p, e, subOpt); err != nil {
			return err
		}
		ret.SetMapIndex(reflect.ValueOf(key), e)
	}

	v.Set(ret)
	return nil
}

func bindStruct(p Properties, v reflect.Value, opt bindOption) error {

	if opt.hasDef && opt.def != "" {
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

// validTag 是否为 ${key:=def} 格式的字符串。
func validTag(tag string) bool {
	return strings.HasPrefix(tag, "${") && strings.HasSuffix(tag, "}")
}

// parseTag 解析 ${key:=def} 字符串，返回 key 和 def 的值。
func parseTag(tag string) (key string, def string, hasDef bool) {
	ss := strings.SplitN(tag[2:len(tag)-1], ":=", 2)
	if len(ss) > 1 {
		hasDef = true
		def = ss[1]
	}
	key = ss[0]
	return
}

func resolveString(p Properties, s string) (string, error) {
	if !validTag(s) {
		return s, nil
	}
	key, def, hasDef := parseTag(s)
	opt := bindOption{key: key, def: def, hasDef: hasDef}
	return resolve(p, opt)
}

func resolve(p Properties, opt bindOption) (string, error) {
	var val string

	if p.Has(opt.key) {
		val = p.Get(opt.key)
	} else if opt.hasDef {
		val = opt.def
	} else {
		return "", fmt.Errorf("property %q %w", opt.key, ErrNotExist)
	}

	return resolveString(p, val)
}
