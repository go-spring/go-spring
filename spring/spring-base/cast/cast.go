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

// Package cast 提供了很多类型之间相互转换的函数。
// Thanks the github.com/spf13/cast project.
package cast

import (
	"fmt"
	"html/template"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cast"
)

// ToBool casts an interface{} to a bool.
func ToBool(i interface{}) bool {
	v, _ := ToBoolE(i)
	return v
}

// ToBoolE casts an interface{} to a bool.
func ToBoolE(i interface{}) (bool, error) {
	if i == nil {
		return false, nil
	}
	switch b := i.(type) {
	case bool:
		return b, nil
	case *bool:
		return *b, nil
	case nil:
		return false, nil
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8,
		*int, *int64, *int32, *int16, *int8, *uint, *uint64, *uint32, *uint16, *uint8:
		return ToInt64(i) != 0, nil
	case float32, float64,
		*float32, *float64:
		return ToFloat64(i) != 0, nil
	case string, *string:
		return strconv.ParseBool(ToString(i))
	default:
		return false, fmt.Errorf("unable to cast %#v of type %T to bool", i, i)
	}
}

// ToInt casts an interface{} to an int.
func ToInt(i interface{}) int {
	v, _ := ToInt64E(i)
	return int(v)
}

// ToInt64 casts an interface{} to an int64.
func ToInt64(i interface{}) int64 {
	v, _ := ToInt64E(i)
	return v
}

// ToInt64E casts an interface{} to an int64.
func ToInt64E(i interface{}) (int64, error) {
	if i == nil {
		return 0, nil
	}
	switch s := i.(type) {
	case int:
		return int64(s), nil
	case *int:
		return int64(*s), nil
	case int64:
		return s, nil
	case *int64:
		return *s, nil
	case int32:
		return int64(s), nil
	case *int32:
		return int64(*s), nil
	case int16:
		return int64(s), nil
	case *int16:
		return int64(*s), nil
	case int8:
		return int64(s), nil
	case *int8:
		return int64(*s), nil
	case uint:
		return int64(s), nil
	case *uint:
		return int64(*s), nil
	case uint64:
		return int64(s), nil
	case *uint64:
		return int64(*s), nil
	case uint32:
		return int64(s), nil
	case *uint32:
		return int64(*s), nil
	case uint16:
		return int64(s), nil
	case *uint16:
		return int64(*s), nil
	case uint8:
		return int64(s), nil
	case *uint8:
		return int64(*s), nil
	case float64:
		return int64(s), nil
	case *float64:
		return int64(*s), nil
	case float32:
		return int64(s), nil
	case *float32:
		return int64(*s), nil
	case string, *string:
		v, err := strconv.ParseInt(ToString(s), 0, 0)
		if err == nil {
			return v, nil
		}
		return 0, fmt.Errorf("unable to cast %#v of type %T to int64", i, i)
	case bool, *bool:
		if ToBool(i) {
			return 1, nil
		}
		return 0, nil
	case nil:
		return 0, nil
	default:
		return 0, fmt.Errorf("unable to cast %#v of type %T to int64", i, i)
	}
}

// ToUint64 casts an interface{} to a uint64.
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface{} to a uint64.
func ToUint64E(i interface{}) (uint64, error) {
	return cast.ToUint64E(i)
}

// ToFloat64 casts an interface{} to a float64.
func ToFloat64(i interface{}) float64 {
	v, _ := ToFloat64E(i)
	return v
}

// ToFloat64E casts an interface{} to a float64.
func ToFloat64E(i interface{}) (float64, error) {
	if i == nil {
		return 0, nil
	}
	switch s := i.(type) {
	case float64:
		return s, nil
	case *float64:
		return *s, nil
	case float32:
		return float64(s), nil
	case *float32:
		return float64(*s), nil
	case int:
		return float64(s), nil
	case *int:
		return float64(*s), nil
	case int64:
		return float64(s), nil
	case *int64:
		return float64(*s), nil
	case int32:
		return float64(s), nil
	case *int32:
		return float64(*s), nil
	case int16:
		return float64(s), nil
	case *int16:
		return float64(*s), nil
	case int8:
		return float64(s), nil
	case *int8:
		return float64(*s), nil
	case uint:
		return float64(s), nil
	case *uint:
		return float64(*s), nil
	case uint64:
		return float64(s), nil
	case *uint64:
		return float64(*s), nil
	case uint32:
		return float64(s), nil
	case *uint32:
		return float64(*s), nil
	case uint16:
		return float64(s), nil
	case *uint16:
		return float64(*s), nil
	case uint8:
		return float64(s), nil
	case *uint8:
		return float64(*s), nil
	case string:
		return strconv.ParseFloat(s, 64)
	case *string:
		return strconv.ParseFloat(*s, 64)
	case bool:
		if s {
			return 1, nil
		}
		return 0, nil
	case *bool:
		if *s {
			return 1, nil
		}
		return 0, nil
	default:
		return 0, fmt.Errorf("unable to cast %#v of type %T to float64", i, i)
	}
}

// ToString casts an interface{} to a string.
func ToString(i interface{}) string {
	// interface 转 string
	result, err := ToStringE(i)
	if err != nil {
		return err.Error()
	}
	return result
}

// ToStringE casts an interface{} to a string.
func ToStringE(i interface{}) (string, error) {
	if i == nil {
		return "", nil
	}
	switch s := i.(type) {
	case string:
		return s, nil
	case *string:
		return *s, nil
	case bool:
		return strconv.FormatBool(s), nil
	case *bool:
		return strconv.FormatBool(*s), nil
	case float64:
		return strconv.FormatFloat(s, 'f', -1, 64), nil
	case *float64:
		return strconv.FormatFloat(*s, 'f', -1, 64), nil
	case float32:
		return strconv.FormatFloat(float64(s), 'f', -1, 32), nil
	case *float32:
		return strconv.FormatFloat(float64(*s), 'f', -1, 32), nil
	case int:
		return strconv.Itoa(s), nil
	case *int:
		return strconv.Itoa(*s), nil
	case int64:
		return strconv.FormatInt(s, 10), nil
	case *int64:
		return strconv.FormatInt(*s, 10), nil
	case int32:
		return strconv.Itoa(int(s)), nil
	case *int32:
		return strconv.Itoa(int(*s)), nil
	case int16:
		return strconv.FormatInt(int64(s), 10), nil
	case *int16:
		return strconv.FormatInt(int64(*s), 10), nil
	case int8:
		return strconv.FormatInt(int64(s), 10), nil
	case *int8:
		return strconv.FormatInt(int64(*s), 10), nil
	case uint:
		return strconv.FormatUint(uint64(s), 10), nil
	case *uint:
		return strconv.FormatUint(uint64(*s), 10), nil
	case uint64:
		return strconv.FormatUint(s, 10), nil
	case *uint64:
		return strconv.FormatUint(*s, 10), nil
	case uint32:
		return strconv.FormatUint(uint64(s), 10), nil
	case *uint32:
		return strconv.FormatUint(uint64(*s), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(s), 10), nil
	case *uint16:
		return strconv.FormatUint(uint64(*s), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(s), 10), nil
	case *uint8:
		return strconv.FormatUint(uint64(*s), 10), nil
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

// ToDuration casts an interface{} to a time.Duration.
func ToDuration(i interface{}) time.Duration {
	v, _ := ToDurationE(i)
	return v
}

// ToDurationE casts an interface{} to a time.Duration.
func ToDurationE(i interface{}) (time.Duration, error) {
	if i == nil {
		return 0, nil
	}
	switch s := i.(type) {
	case time.Duration:
		return s, nil
	case int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8,
		*int, *int64, *int32, *int16, *int8, *uint, *uint64, *uint32, *uint16, *uint8:
		return time.Duration(ToInt64(s)), nil
	case float32, float64,
		*float32, *float64:
		return time.Duration(ToFloat64(s)), nil
	case string, *string:
		v := ToString(s)
		if strings.ContainsAny(v, "nsuµmh") {
			return time.ParseDuration(v)
		} else {
			return time.ParseDuration(v + "ns")
		}
	default:
		return 0, fmt.Errorf("unable to cast %#v of type %T to Duration", i, i)
	}
}

// ToTime casts an interface{} to a time.Time.
func ToTime(i interface{}) time.Time {
	v, _ := ToTimeE(i)
	return v
}

// ToTimeE casts an interface{} to a time.Time.
func ToTimeE(i interface{}) (time.Time, error) {
	return cast.ToTimeE(i)
}

// ToStringSlice casts an interface to a []string type.
func ToStringSlice(i interface{}) []string {
	v, _ := ToStringSliceE(i)
	return v
}

// ToStringSliceE casts an interface to a []string type.
func ToStringSliceE(i interface{}) ([]string, error) {
	// TODO 使用具体的类型判断，看看是否有更好的性能。
	switch v := reflect.ValueOf(i); v.Kind() {
	case reflect.Slice, reflect.Array:
		var slice []string
		for j := 0; j < v.Len(); j++ {
			s := ToString(v.Index(j).Interface())
			slice = append(slice, s)
		}
		return slice, nil
	}
	return nil, fmt.Errorf("unable to cast %#v of type %T to []string", i, i)
}
