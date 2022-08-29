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
	"github.com/go-spring/spring-base/util"
)

var (
	errNotExist      = errors.New("not exist")
	errInvalidSyntax = errors.New("invalid syntax")
)

// ParsedTag a value tag includes at most three parts: required key, optional
// default value, and optional splitter, the syntax is ${key:=value}||splitter.
type ParsedTag struct {
	Key      string // short property key
	Def      string // default value
	HasDef   bool   // has default value
	Splitter string // splitter's name
}

// ParseTag parses a value tag, returns its key, and default value, and splitter.
func ParseTag(tag string) (ret ParsedTag, err error) {
	i := strings.LastIndex(tag, "||")
	if i == 0 {
		err = errInvalidSyntax
		err = util.Wrapf(err, code.FileLine(), "parse tag %q error", tag)
		return
	}
	j := strings.LastIndex(tag, "}")
	if j <= 0 {
		err = errInvalidSyntax
		err = util.Wrapf(err, code.FileLine(), "parse tag %q error", tag)
		return
	}
	k := strings.Index(tag, "${")
	if k < 0 {
		err = errInvalidSyntax
		err = util.Wrapf(err, code.FileLine(), "parse tag %q error", tag)
		return
	}
	if i > j {
		ret.Splitter = strings.TrimSpace(tag[i+2:])
	}
	ss := strings.SplitN(tag[k+2:j], ":=", 2)
	ret.Key = ss[0]
	if len(ss) > 1 {
		ret.HasDef = true
		ret.Def = ss[1]
	}
	return
}

type BindParam struct {
	Type reflect.Type // reflection type of binding value
	Key  string       // full property key
	Path string       // binding path
	tag  ParsedTag    // parsed tag
}

func (param *BindParam) Tag() ParsedTag {
	return param.tag
}

func (param *BindParam) BindTag(tag string) error {
	parsedTag, err := ParseTag(tag)
	if err != nil {
		return err
	}
	if parsedTag.Key == "ROOT" {
		parsedTag.Key = ""
	} else if parsedTag.Key == "" {
		parsedTag.Key = "ANONYMOUS"
	}
	param.tag = parsedTag
	if param.Key == "" {
		param.Key = parsedTag.Key
	} else if parsedTag.Key != "" {
		param.Key = param.Key + "." + parsedTag.Key
	}
	return nil
}

// BindValue binds properties to a value.
func BindValue(p *Properties, v reflect.Value, param BindParam) error {

	if !util.IsValueType(param.Type) {
		err := errors.New("target should be value type")
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	}

	switch v.Kind() {
	case reflect.Map:
		return bindMap(p, v, param)
	case reflect.Array:
		err := errors.New("use slice instead of array")
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	case reflect.Slice:
		return bindSlice(p, v, param)
	}

	fn := converters[param.Type]
	if fn == nil && v.Kind() == reflect.Struct {
		if err := bindStruct(p, v, param); err != nil {
			return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
		}
		return nil
	}

	val, err := resolve(p, param)
	if err != nil {
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	}

	if fn != nil {
		fnValue := reflect.ValueOf(fn)
		out := fnValue.Call([]reflect.Value{reflect.ValueOf(val)})
		if !out[1].IsNil() {
			err = out[1].Interface().(error)
			return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
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
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int64
		if i, err = strconv.ParseInt(val, 0, 0); err == nil {
			v.SetInt(i)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(val, 64); err == nil {
			v.SetFloat(f)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(val); err == nil {
			v.SetBool(b)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	case reflect.String:
		v.SetString(val)
		return nil
	}

	err = fmt.Errorf("unsupported bind type %q", param.Type.String())
	return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
}

// bindSlice binds properties to a slice value.
func bindSlice(p *Properties, v reflect.Value, param BindParam) error {

	et := param.Type.Elem()
	p, err := getSlice(p, et, param)
	if err != nil {
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	}
	if p == nil {
		return nil
	}

	slice := reflect.MakeSlice(param.Type, 0, 0)
	for i := 0; ; i++ {
		e := reflect.New(et).Elem()
		subParam := BindParam{
			Type: et,
			Key:  fmt.Sprintf("%s[%d]", param.Key, i),
			Path: fmt.Sprintf("%s[%d]", param.Path, i),
		}
		err = BindValue(p, e, subParam)
		if errors.Is(err, errNotExist) {
			break
		}
		if err != nil {
			return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
		}
		slice = reflect.Append(slice, e)
	}
	v.Set(slice)
	return nil
}

func getSlice(p *Properties, et reflect.Type, param BindParam) (*Properties, error) {

	// properties defined as list.
	if p.Has(param.Key + "[0]") {
		return p, nil
	}

	// properties defined as string and needs to split into []string.
	var strVal string
	{
		if p.Has(param.Key) {
			strVal = p.Get(param.Key)
		} else {
			if !param.tag.HasDef {
				return nil, util.Errorf(code.FileLine(), "property %q %w", param.Key, errNotExist)
			}
			if param.tag.Def == "" {
				return nil, nil
			}
			if !util.IsPrimitiveValueType(et) && converters[et] == nil {
				return nil, util.Error(code.FileLine(), "slice can't have a non empty default value")
			}
			strVal = param.tag.Def
		}
	}
	if strVal == "" {
		return nil, nil
	}

	var (
		err    error
		arrVal []string
	)

	if s := param.tag.Splitter; s == "" {
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

// bindMap binds properties to a map value.
func bindMap(p *Properties, v reflect.Value, param BindParam) error {

	if param.tag.HasDef && param.tag.Def != "" {
		err := errors.New("map can't have a non empty default value")
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
	}

	et := param.Type.Elem()
	ret := reflect.MakeMap(param.Type)
	defer v.Set(ret)

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
				if param.tag.Def == "" {
					return nil
				}
				err := fmt.Errorf("property %q %w", param.Key, errNotExist)
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
			}
			_, ok = vt.(struct{})
			if ok {
				if i == len(keyPath)-1 {
					t = nil
					break
				}
				oldKey := strings.Join(keyPath[:i+1], ".")
				err := fmt.Errorf("property %q isn't map", oldKey)
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
			}
			t = vt.(map[string]interface{})
		}
		for k := range t {
			keys = append(keys, k)
		}
	}

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
			return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
		}
		ret.SetMapIndex(reflect.ValueOf(key), e)
	}
	return nil
}

// bindStruct binds properties to a struct value.
func bindStruct(p *Properties, v reflect.Value, param BindParam) error {

	if param.tag.HasDef && param.tag.Def != "" {
		err := errors.New("struct can't have a non empty default value")
		return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
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
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
			}
			if err := BindValue(p, fv, subParam); err != nil {
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
			}
			continue
		}

		if ft.Anonymous {
			// embed pointer type may lead to infinite recursion.
			if ft.Type.Kind() != reflect.Struct {
				continue
			}
			if err := bindStruct(p, fv, subParam); err != nil {
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
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
				return util.Wrapf(err, code.FileLine(), "bind %s error", param.Path)
			}
		}
	}
	return nil
}

// resolve returns property references processed property value.
func resolve(p *Properties, param BindParam) (string, error) {
	if val, ok := p.data[param.Key]; ok {
		return resolveString(p, val)
	}
	if param.tag.HasDef {
		return resolveString(p, param.tag.Def)
	}
	err := fmt.Errorf("property %q %w", param.Key, errNotExist)
	return "", util.Wrapf(err, code.FileLine(), "resolve property %q error", param.Key)
}

// resolveString returns property references processed string.
func resolveString(p *Properties, s string) (string, error) {

	var (
		length = len(s)
		count  = 0
		start  = -1
		end    = -1
	)

	for i := 0; i < length; i++ {
		if s[i] == '$' {
			if i < length-1 && s[i+1] == '{' {
				if count == 0 {
					start = i
				}
				count++
			}
		} else if s[i] == '}' {
			if count > 0 {
				count--
				if count == 0 {
					end = i
					break
				}
			}
		}
	}

	if start < 0 {
		return s, nil
	}

	if end < 0 || count > 0 {
		err := errInvalidSyntax
		return "", util.Wrapf(err, code.FileLine(), "resolve string %q error", s)
	}

	var param BindParam
	err := param.BindTag(s[start : end+1])
	if err != nil {
		return "", util.Wrapf(err, code.FileLine(), "resolve string %q error", s)
	}

	s1, err := resolve(p, param)
	if err != nil {
		return "", util.Wrapf(err, code.FileLine(), "resolve string %q error", s)
	}

	s2, err := resolveString(p, s[end+1:])
	if err != nil {
		return "", util.Wrapf(err, code.FileLine(), "resolve string %q error", s)
	}

	return s[:start] + s1 + s2, nil
}
