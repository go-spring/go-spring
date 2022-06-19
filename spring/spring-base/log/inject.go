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

package log

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/code"
	"github.com/go-spring/spring-base/util"
)

func inject(v reflect.Value, t reflect.Type, node *Node) error {
	for i := 0; i < v.NumField(); i++ {
		ft := t.Field(i)
		fv := v.Field(i)
		if tag, ok := ft.Tag.Lookup("PluginAttribute"); ok {
			if err := injectAttribute(tag, fv, ft, node); err != nil {
				return err
			}
			continue
		}
		if tag, ok := ft.Tag.Lookup("PluginElement"); ok {
			if err := injectElement(tag, fv, ft, node); err != nil {
				return err
			}
		}
		if ft.Anonymous && ft.Type.Kind() == reflect.Struct {
			if err := inject(fv, fv.Type(), node); err != nil {
				return err
			}
		}
	}
	return nil
}

func injectAttribute(tag string, fv reflect.Value, ft reflect.StructField, node *Node) error {

	var (
		err   error
		attrs map[string]string
	)

	if ss := strings.Split(tag, ","); len(ss) > 0 {
		tag = ss[0]
		attrs = make(map[string]string)
		for j := 1; j < len(ss); j++ {
			rs := strings.Split(ss[j], "=")
			if len(rs) > 1 {
				attrs[rs[0]] = rs[1]
			} else {
				attrs[rs[0]] = ""
			}
		}
	}

	val, ok := node.Attributes[tag]
	if !ok {
		val, ok = attrs["default"]
		if !ok {
			return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
		}
	}

	if ft.Name == "Level" {
		fv.Set(reflect.ValueOf(StringToLevel(val)))
		return nil
	}

	switch fv.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var u uint64
		if u, err = strconv.ParseUint(val, 0, 0); err == nil {
			fv.SetUint(u)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "inject %s error", ft.Name)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		var i int64
		if i, err = strconv.ParseInt(val, 0, 0); err == nil {
			fv.SetInt(i)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "inject %s error", ft.Name)
	case reflect.Float32, reflect.Float64:
		var f float64
		if f, err = strconv.ParseFloat(val, 64); err == nil {
			fv.SetFloat(f)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "inject %s error", ft.Name)
	case reflect.Bool:
		var b bool
		if b, err = strconv.ParseBool(val); err == nil {
			fv.SetBool(b)
			return nil
		}
		return util.Wrapf(err, code.FileLine(), "inject %s error", ft.Name)
	case reflect.String:
		fv.SetString(val)
		return nil
	}
	return nil
}

func injectElement(tag string, fv reflect.Value, ft reflect.StructField, node *Node) error {
	var children []reflect.Value
	for _, childNode := range node.Children {
		if p := getPluginByName(childNode.Label); p != nil {
			if p.Type != tag {
				continue
			}
			pv := reflect.New(p.Class)
			err := inject(pv.Elem(), pv.Type().Elem(), childNode)
			if err != nil {
				return err
			}
			children = append(children, pv)
		}
	}
	if len(children) == 0 {
		return nil
	}
	switch fv.Kind() {
	case reflect.Slice:
		slice := reflect.MakeSlice(ft.Type, 0, 0)
		for j := 0; j < len(children); j++ {
			slice = reflect.Append(slice, children[j])
		}
		fv.Set(slice)
	case reflect.Interface:
		if len(children) > 1 {
			return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
		}
		fv.Set(children[0])
	default:
		return fmt.Errorf("xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
	}
	fmt.Println(children)
	return nil
}
