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

package cast

import (
	"bytes"
	"fmt"
	"html/template"
	"strconv"

	"github.com/go-spring/spring-base/fastdev/json"
)

// ToString casts an interface{} to a string. 在类型明确的情况下推荐使用标准库函数。
func ToString(i interface{}) string {
	v, _ := ToStringE(i)
	return v
}

// ToStringE casts an interface{} to a string. 在类型明确的情况下推荐使用标准库函数。
func ToStringE(i interface{}) (string, error) {
	switch s := i.(type) {
	case nil:
		return "", nil
	case int:
		return strconv.Itoa(s), nil
	case int8:
		return strconv.FormatInt(int64(s), 10), nil
	case int16:
		return strconv.FormatInt(int64(s), 10), nil
	case int32:
		return strconv.Itoa(int(s)), nil
	case int64:
		return strconv.FormatInt(s, 10), nil
	case *int:
		return strconv.Itoa(*s), nil
	case *int8:
		return strconv.FormatInt(int64(*s), 10), nil
	case *int16:
		return strconv.FormatInt(int64(*s), 10), nil
	case *int32:
		return strconv.Itoa(int(*s)), nil
	case *int64:
		return strconv.FormatInt(*s, 10), nil
	case uint:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(s), 10), nil
	case uint64:
		return strconv.FormatUint(s, 10), nil
	case *uint:
		return strconv.FormatUint(uint64(*s), 10), nil
	case *uint8:
		return strconv.FormatUint(uint64(*s), 10), nil
	case *uint16:
		return strconv.FormatUint(uint64(*s), 10), nil
	case *uint32:
		return strconv.FormatUint(uint64(*s), 10), nil
	case *uint64:
		return strconv.FormatUint(*s, 10), nil
	case float32:
		return strconv.FormatFloat(float64(s), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64), nil
	case *float32:
		return strconv.FormatFloat(float64(*s), 'f', -1, 32), nil
	case *float64:
		return strconv.FormatFloat(*s, 'f', -1, 64), nil
	case string:
		return s, nil
	case *string:
		return *s, nil
	case bool:
		return strconv.FormatBool(s), nil
	case *bool:
		return strconv.FormatBool(*s), nil
	case []byte:
		return string(s), nil
	case template.HTML:
		return string(s), nil
	case template.URL:
		return string(s), nil
	case template.JS:
		return string(s), nil
	case template.CSS:
		return string(s), nil
	case template.HTMLAttr:
		return string(s), nil
	case fmt.Stringer:
		return s.String(), nil
	case error:
		return s.Error(), nil
	default:
		return "", fmt.Errorf("unable to cast %#v of type %T to string", i, i)
	}
}

// ToStringSlice casts an interface{} to a []string.
func ToStringSlice(i interface{}) []string {
	v, _ := ToStringSliceE(i)
	return v
}

// ToStringSliceE casts an interface{} to a []string.
func ToStringSliceE(i interface{}) ([]string, error) {
	switch v := i.(type) {
	case nil:
		return nil, nil
	case []string:
		return v, nil
	case []int:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.Itoa(v[j])
			slice = append(slice, s)
		}
		return slice, nil
	case []int8:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatInt(int64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []int16:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatInt(int64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []int32:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatInt(int64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []int64:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatInt(v[j], 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []uint:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatUint(uint64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []uint8:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatUint(uint64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []uint16:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatUint(uint64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []uint32:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatUint(uint64(v[j]), 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []uint64:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatUint(v[j], 10)
			slice = append(slice, s)
		}
		return slice, nil
	case []bool:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatBool(v[j])
			slice = append(slice, s)
		}
		return slice, nil
	case []float32:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatFloat(float64(v[j]), 'f', -1, 32)
			slice = append(slice, s)
		}
		return slice, nil
	case []float64:
		var slice []string
		for j := 0; j < len(v); j++ {
			s := strconv.FormatFloat(v[j], 'f', -1, 64)
			slice = append(slice, s)
		}
		return slice, nil
	case []interface{}:
		var slice []string
		for j := 0; j < len(v); j++ {
			s, err := ToStringE(v[j])
			if err != nil {
				return nil, err
			}
			slice = append(slice, s)
		}
		return slice, nil
	}
	return nil, fmt.Errorf("unable to cast %#v of type %T to []string", i, i)
}

// ToStringMap casts an interface{} to a map[string]interface{}.
func ToStringMap(i interface{}) map[string]interface{} {
	v, _ := ToStringMapE(i)
	return v
}

// ToStringMapE casts an interface{} to a map[string]interface{}.
func ToStringMapE(i interface{}) (map[string]interface{}, error) {
	switch v := i.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		return v, nil
	case map[interface{}]interface{}:
		var m = map[string]interface{}{}
		for key, val := range v {
			k, err := ToStringE(key)
			if err != nil {
				return nil, err
			}
			m[k] = val
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unable to cast %#v of type %T to map[string]interface{}", i, i)
	}
}

// ToStringMapString casts an interface{} to a map[string]string.
func ToStringMapString(i interface{}) map[string]string {
	v, _ := ToStringMapStringE(i)
	return v
}

// ToStringMapStringE casts an interface{} to a map[string]string.
func ToStringMapStringE(i interface{}) (map[string]string, error) {
	switch v := i.(type) {
	case nil:
		return nil, nil
	case map[string]string:
		return v, nil
	case map[string]interface{}:
		var err error
		var m = map[string]string{}
		for key, val := range v {
			m[key], err = ToStringE(val)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	case map[interface{}]string:
		var m = map[string]string{}
		for key, val := range v {
			k, err := ToStringE(key)
			if err != nil {
				return nil, err
			}
			m[k] = val
		}
		return m, nil
	case map[interface{}]interface{}:
		var m = map[string]string{}
		for key, val := range v {
			k, err := ToStringE(key)
			if err != nil {
				return nil, err
			}
			m[k], err = ToStringE(val)
			if err != nil {
				return nil, err
			}
		}
		return m, nil
	default:
		return nil, fmt.Errorf("unable to cast %#v of type %T to map[string]string", i, i)
	}
}

// needQuote 判断是否需要双引号包裹。
func needQuote(s string) bool {
	for _, c := range s {
		switch c {
		case '"', '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0:
			return true
		}
	}
	return len(s) == 0
}

func quoteString(s string) string {
	if needQuote(s) || json.NeedQuote(s) {
		return json.Quote(s)
	}
	return s
}

func CmdString(args []interface{}) string {
	var buf bytes.Buffer
	for i, arg := range args {
		switch s := arg.(type) {
		case string:
			buf.WriteString(quoteString(s))
		default:
			buf.WriteString(ToString(arg))
		}
		if i < len(args)-1 {
			buf.WriteByte(' ')
		}
	}
	return buf.String()
}
