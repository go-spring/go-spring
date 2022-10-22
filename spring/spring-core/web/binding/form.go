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

package binding

import (
	"net/url"
	"reflect"
)

func BindForm(i interface{}, r Request) error {
	params, err := r.FormParams()
	if err != nil {
		return err
	}
	t := reflect.TypeOf(i)
	if t.Kind() != reflect.Ptr {
		return nil
	}
	et := t.Elem()
	if et.Kind() != reflect.Struct {
		return nil
	}
	ev := reflect.ValueOf(i).Elem()
	return bindFormStruct(ev, et, params)
}

func bindFormStruct(v reflect.Value, t reflect.Type, params url.Values) error {
	for j := 0; j < t.NumField(); j++ {
		ft := t.Field(j)
		fv := v.Field(j)
		if ft.Anonymous {
			if ft.Type.Kind() != reflect.Struct {
				continue
			}
			err := bindFormStruct(fv, ft.Type, params)
			if err != nil {
				return err
			}
			continue
		}
		name, ok := ft.Tag.Lookup("form")
		if !ok || !fv.CanInterface() {
			continue
		}
		values := params[name]
		if len(values) == 0 {
			continue
		}
		err := bindFormField(fv, ft.Type, values)
		if err != nil {
			return err
		}
	}
	return nil
}

func bindFormField(v reflect.Value, t reflect.Type, values []string) error {
	if v.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(t, 0, len(values))
		defer func() { v.Set(slice) }()
		et := t.Elem()
		for _, value := range values {
			ev := reflect.New(et).Elem()
			err := bindData(ev, value)
			if err != nil {
				return err
			}
			slice = reflect.Append(slice, ev)
		}
		return nil
	}
	return bindData(v, values[0])
}
