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
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"
)

// Group 返回指定前缀的属性值集合并进行分组，属性名称统一转成小写。
func Group(key string, m map[string]interface{}) map[string]map[string]interface{} {
	key = strings.ToLower(key) + "."
	result := make(map[string]map[string]interface{})
	for k, v := range m {
		if strings.HasPrefix(k, key) {
			ss := strings.SplitN(k[len(key):], ".", 2)
			group := ss[0]
			m, ok := result[group]
			if !ok {
				m = make(map[string]interface{})
				result[group] = m
			}
			m[k] = v
		}
	}
	return result
}

// ToIntSliceE casts an interface to a []int type.
func ToIntSliceE(i interface{}) ([]int, error) {
	if i == nil {
		return []int{}, fmt.Errorf("unable to cast %#v of type %T to []int", i, i)
	}

	switch v := i.(type) {
	case []int:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]int, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToIntE(s.Index(j).Interface())
			if err != nil {
				return []int{}, fmt.Errorf("unable to cast %#v of type %T to []int", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []int{}, fmt.Errorf("unable to cast %#v of type %T to []int", i, i)
	}
}

// ToInt8SliceE casts an interface to a []int8 type.
func ToInt8SliceE(i interface{}) ([]int8, error) {
	if i == nil {
		return []int8{}, fmt.Errorf("unable to cast %#v of type %T to []int8", i, i)
	}

	switch v := i.(type) {
	case []int8:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]int8, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToInt8E(s.Index(j).Interface())
			if err != nil {
				return []int8{}, fmt.Errorf("unable to cast %#v of type %T to []int8", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []int8{}, fmt.Errorf("unable to cast %#v of type %T to []int8", i, i)
	}
}

// ToInt16SliceE casts an interface to a []int16 type.
func ToInt16SliceE(i interface{}) ([]int16, error) {
	if i == nil {
		return []int16{}, fmt.Errorf("unable to cast %#v of type %T to []int16", i, i)
	}

	switch v := i.(type) {
	case []int16:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]int16, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToInt16E(s.Index(j).Interface())
			if err != nil {
				return []int16{}, fmt.Errorf("unable to cast %#v of type %T to []int16", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []int16{}, fmt.Errorf("unable to cast %#v of type %T to []int16", i, i)
	}
}

// ToInt32SliceE casts an interface to a []int32 type.
func ToInt32SliceE(i interface{}) ([]int32, error) {
	if i == nil {
		return []int32{}, fmt.Errorf("unable to cast %#v of type %T to []int32", i, i)
	}

	switch v := i.(type) {
	case []int32:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]int32, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToInt32E(s.Index(j).Interface())
			if err != nil {
				return []int32{}, fmt.Errorf("unable to cast %#v of type %T to []int32", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []int32{}, fmt.Errorf("unable to cast %#v of type %T to []int32", i, i)
	}
}

// ToInt64SliceE casts an interface to a []int64 type.
func ToInt64SliceE(i interface{}) ([]int64, error) {
	if i == nil {
		return []int64{}, fmt.Errorf("unable to cast %#v of type %T to []int64", i, i)
	}

	switch v := i.(type) {
	case []int64:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]int64, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToInt64E(s.Index(j).Interface())
			if err != nil {
				return []int64{}, fmt.Errorf("unable to cast %#v of type %T to []int64", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []int64{}, fmt.Errorf("unable to cast %#v of type %T to []int64", i, i)
	}
}

// ToUintSliceE casts an interface to a []uint type.
func ToUintSliceE(i interface{}) ([]uint, error) {
	if i == nil {
		return []uint{}, fmt.Errorf("unable to cast %#v of type %T to []uint", i, i)
	}

	switch v := i.(type) {
	case []uint:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]uint, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToUintE(s.Index(j).Interface())
			if err != nil {
				return []uint{}, fmt.Errorf("unable to cast %#v of type %T to []uint", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []uint{}, fmt.Errorf("unable to cast %#v of type %T to []uint", i, i)
	}
}

// ToUint8SliceE casts an interface to a []uint8 type.
func ToUint8SliceE(i interface{}) ([]uint8, error) {
	if i == nil {
		return []uint8{}, fmt.Errorf("unable to cast %#v of type %T to []uint8", i, i)
	}

	switch v := i.(type) {
	case []uint8:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]uint8, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToUint8E(s.Index(j).Interface())
			if err != nil {
				return []uint8{}, fmt.Errorf("unable to cast %#v of type %T to []uint8", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []uint8{}, fmt.Errorf("unable to cast %#v of type %T to []uint", i, i)
	}
}

// ToUint16SliceE casts an interface to a []uint16 type.
func ToUint16SliceE(i interface{}) ([]uint16, error) {
	if i == nil {
		return []uint16{}, fmt.Errorf("unable to cast %#v of type %T to []uint16", i, i)
	}

	switch v := i.(type) {
	case []uint16:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]uint16, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToUint16E(s.Index(j).Interface())
			if err != nil {
				return []uint16{}, fmt.Errorf("unable to cast %#v of type %T to []uint16", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []uint16{}, fmt.Errorf("unable to cast %#v of type %T to []uint16", i, i)
	}
}

// ToUint32SliceE casts an interface to a []uint32 type.
func ToUint32SliceE(i interface{}) ([]uint32, error) {
	if i == nil {
		return []uint32{}, fmt.Errorf("unable to cast %#v of type %T to []uint32", i, i)
	}

	switch v := i.(type) {
	case []uint32:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]uint32, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToUint32E(s.Index(j).Interface())
			if err != nil {
				return []uint32{}, fmt.Errorf("unable to cast %#v of type %T to []uint32", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []uint32{}, fmt.Errorf("unable to cast %#v of type %T to []uint32", i, i)
	}
}

// ToUint64SliceE casts an interface to a []uint64 type.
func ToUint64SliceE(i interface{}) ([]uint64, error) {
	if i == nil {
		return []uint64{}, fmt.Errorf("unable to cast %#v of type %T to []uint64", i, i)
	}

	switch v := i.(type) {
	case []uint64:
		return v, nil
	}

	kind := reflect.TypeOf(i).Kind()
	switch kind {
	case reflect.Slice, reflect.Array:
		s := reflect.ValueOf(i)
		a := make([]uint64, s.Len())
		for j := 0; j < s.Len(); j++ {
			val, err := cast.ToUint64E(s.Index(j).Interface())
			if err != nil {
				return []uint64{}, fmt.Errorf("unable to cast %#v of type %T to []uint64", i, i)
			}
			a[j] = val
		}
		return a, nil
	default:
		return []uint64{}, fmt.Errorf("unable to cast %#v of type %T to []uint64", i, i)
	}
}
