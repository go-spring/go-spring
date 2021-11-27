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
	"time"
)

// ToBool casts an interface{} to a bool.
func ToBool(i interface{}) bool {
	v, _ := ToBoolE(i)
	return v
}

// ToBoolE casts an interface{} to a bool.
func ToBoolE(i interface{}) (bool, error) {
	switch b := i.(type) {
	case nil:
		return false, nil
	case int:
		return b != 0, nil
	case int8:
		return b != 0, nil
	case int16:
		return b != 0, nil
	case int32:
		return b != 0, nil
	case int64:
		return b != 0, nil
	case *int:
		return *b != 0, nil
	case *int8:
		return *b != 0, nil
	case *int16:
		return *b != 0, nil
	case *int32:
		return *b != 0, nil
	case *int64:
		return *b != 0, nil
	case uint:
		return b != 0, nil
	case uint8:
		return b != 0, nil
	case uint16:
		return b != 0, nil
	case uint32:
		return b != 0, nil
	case uint64:
		return b != 0, nil
	case *uint:
		return *b != 0, nil
	case *uint8:
		return *b != 0, nil
	case *uint16:
		return *b != 0, nil
	case *uint32:
		return *b != 0, nil
	case *uint64:
		return *b != 0, nil
	case float32:
		return b != 0, nil
	case float64:
		return b != 0, nil
	case *float32:
		return *b != 0, nil
	case *float64:
		return *b != 0, nil
	case string:
		return strconv.ParseBool(b)
	case *string:
		return strconv.ParseBool(*b)
	case bool:
		return b, nil
	case *bool:
		return *b, nil
	default:
		return false, fmt.Errorf("unable to cast %#v of type %T to bool", i, i)
	}
}

// ToInt casts an interface{} to an int.
func ToInt(i interface{}) int {
	v, _ := ToInt64E(i)
	return int(v)
}

// ToInt8 casts an interface{} to an int8.
func ToInt8(i interface{}) int8 {
	v, _ := ToInt64E(i)
	return int8(v)
}

// ToInt16 casts an interface{} to an int16.
func ToInt16(i interface{}) int16 {
	v, _ := ToInt64E(i)
	return int16(v)
}

// ToInt32 casts an interface{} to an int32.
func ToInt32(i interface{}) int32 {
	v, _ := ToInt64E(i)
	return int32(v)
}

// ToInt64 casts an interface{} to an int64.
func ToInt64(i interface{}) int64 {
	v, _ := ToInt64E(i)
	return v
}

// ToInt64E casts an interface{} to an int64.
func ToInt64E(i interface{}) (int64, error) {
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return int64(s), nil
	case int8:
		return int64(s), nil
	case int16:
		return int64(s), nil
	case int32:
		return int64(s), nil
	case int64:
		return s, nil
	case *int:
		return int64(*s), nil
	case *int8:
		return int64(*s), nil
	case *int16:
		return int64(*s), nil
	case *int32:
		return int64(*s), nil
	case *int64:
		return *s, nil
	case uint:
		return int64(s), nil
	case uint8:
		return int64(s), nil
	case uint16:
		return int64(s), nil
	case uint32:
		return int64(s), nil
	case uint64:
		return int64(s), nil
	case *uint:
		return int64(*s), nil
	case *uint8:
		return int64(*s), nil
	case *uint16:
		return int64(*s), nil
	case *uint32:
		return int64(*s), nil
	case *uint64:
		return int64(*s), nil
	case float32:
		return int64(s), nil
	case float64:
		return int64(s), nil
	case *float32:
		return int64(*s), nil
	case *float64:
		return int64(*s), nil
	case string:
		if v, err := strconv.ParseInt(s, 0, 0); err == nil {
			return v, nil
		}
	case *string:
		if v, err := strconv.ParseInt(*s, 0, 0); err == nil {
			return v, nil
		}
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
	}
	return 0, fmt.Errorf("unable to cast %#v of type %T to int64", i, i)
}

// ToUint casts an interface{} to an uint.
func ToUint(i interface{}) uint {
	v, _ := ToUint64E(i)
	return uint(v)
}

// ToUint8 casts an interface{} to an uint8.
func ToUint8(i interface{}) uint8 {
	v, _ := ToUint64E(i)
	return uint8(v)
}

// ToUint16 casts an interface{} to an uint16.
func ToUint16(i interface{}) uint16 {
	v, _ := ToUint64E(i)
	return uint16(v)
}

// ToUint32 casts an interface{} to an uint32.
func ToUint32(i interface{}) uint32 {
	v, _ := ToUint64E(i)
	return uint32(v)
}

// ToUint64 casts an interface{} to an uint64.
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface{} to an uint64.
func ToUint64E(i interface{}) (uint64, error) {
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return uint64(s), nil
	case int8:
		return uint64(s), nil
	case int16:
		return uint64(s), nil
	case int32:
		return uint64(s), nil
	case int64:
		return uint64(s), nil
	case *int:
		return uint64(*s), nil
	case *int8:
		return uint64(*s), nil
	case *int16:
		return uint64(*s), nil
	case *int32:
		return uint64(*s), nil
	case *int64:
		return uint64(*s), nil
	case uint:
		return uint64(s), nil
	case uint8:
		return uint64(s), nil
	case uint16:
		return uint64(s), nil
	case uint32:
		return uint64(s), nil
	case uint64:
		return s, nil
	case *uint:
		return uint64(*s), nil
	case *uint8:
		return uint64(*s), nil
	case *uint16:
		return uint64(*s), nil
	case *uint32:
		return uint64(*s), nil
	case *uint64:
		return *s, nil
	case float32:
		return uint64(s), nil
	case float64:
		return uint64(s), nil
	case *float32:
		return uint64(*s), nil
	case *float64:
		return uint64(*s), nil
	case string:
		if v, err := strconv.ParseUint(s, 0, 0); err == nil {
			return v, nil
		}
	case *string:
		if v, err := strconv.ParseUint(*s, 0, 0); err == nil {
			return v, nil
		}
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
	}
	return 0, fmt.Errorf("unable to cast %#v of type %T to uint64", i, i)
}

// ToFloat32 casts an interface{} to a float32.
func ToFloat32(i interface{}) float32 {
	v, _ := ToFloat64E(i)
	return float32(v)
}

// ToFloat64 casts an interface{} to a float64.
func ToFloat64(i interface{}) float64 {
	v, _ := ToFloat64E(i)
	return v
}

// ToFloat64E casts an interface{} to a float64.
func ToFloat64E(i interface{}) (float64, error) {
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return float64(s), nil
	case int8:
		return float64(s), nil
	case int16:
		return float64(s), nil
	case int32:
		return float64(s), nil
	case int64:
		return float64(s), nil
	case *int:
		return float64(*s), nil
	case *int8:
		return float64(*s), nil
	case *int16:
		return float64(*s), nil
	case *int32:
		return float64(*s), nil
	case *int64:
		return float64(*s), nil
	case uint:
		return float64(s), nil
	case uint8:
		return float64(s), nil
	case uint16:
		return float64(s), nil
	case uint32:
		return float64(s), nil
	case uint64:
		return float64(s), nil
	case *uint:
		return float64(*s), nil
	case *uint8:
		return float64(*s), nil
	case *uint16:
		return float64(*s), nil
	case *uint32:
		return float64(*s), nil
	case *uint64:
		return float64(*s), nil
	case float32:
		return float64(s), nil
	case float64:
		return s, nil
	case *float32:
		return float64(*s), nil
	case *float64:
		return *s, nil
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
	v, _ := ToStringE(i)
	return v
}

// ToStringE casts an interface{} to a string.
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

// ToDuration casts an interface{} to a time.Duration.
func ToDuration(i interface{}) time.Duration {
	v, _ := ToDurationE(i)
	return v
}

// ToDurationE casts an interface{} to a time.Duration.
func ToDurationE(i interface{}) (time.Duration, error) {
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return time.Duration(s), nil
	case int8:
		return time.Duration(s), nil
	case int16:
		return time.Duration(s), nil
	case int32:
		return time.Duration(s), nil
	case int64:
		return time.Duration(s), nil
	case *int:
		return time.Duration(*s), nil
	case *int8:
		return time.Duration(*s), nil
	case *int16:
		return time.Duration(*s), nil
	case *int32:
		return time.Duration(*s), nil
	case *int64:
		return time.Duration(*s), nil
	case uint:
		return time.Duration(s), nil
	case uint8:
		return time.Duration(s), nil
	case uint16:
		return time.Duration(s), nil
	case uint32:
		return time.Duration(s), nil
	case uint64:
		return time.Duration(s), nil
	case *uint:
		return time.Duration(*s), nil
	case *uint8:
		return time.Duration(*s), nil
	case *uint16:
		return time.Duration(*s), nil
	case *uint32:
		return time.Duration(*s), nil
	case *uint64:
		return time.Duration(*s), nil
	case float32:
		return time.Duration(s), nil
	case float64:
		return time.Duration(s), nil
	case *float32:
		return time.Duration(*s), nil
	case *float64:
		return time.Duration(*s), nil
	case string:
		return time.ParseDuration(s)
	case *string:
		return time.ParseDuration(*s)
	case time.Duration:
		return s, nil
	default:
		return 0, fmt.Errorf("unable to cast %#v of type %T to time.Duration", i, i)
	}
}

type TimeArg struct {
	Format string        // 解析日期字符串时使用该参数
	Unit   time.Duration // 解析整数日期值时使用该参数
}

// ToTime casts an interface{} to a time.Time.
func ToTime(i interface{}, arg TimeArg) time.Time {
	v, _ := ToTimeE(i, arg)
	return v
}

// ToTimeE casts an interface{} to a time.Time.
func ToTimeE(i interface{}, arg TimeArg) (time.Time, error) {
	switch v := i.(type) {
	case nil:
		return time.Time{}, nil
	case int:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case int8:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case int16:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case int32:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case int64:
		return parseTimestamp((v) * int64(arg.Unit)), nil
	case *int:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *int8:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *int16:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *int32:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *int64:
		return parseTimestamp((*v) * int64(arg.Unit)), nil
	case uint:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case uint8:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case uint16:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case uint32:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case uint64:
		return parseTimestamp(int64(v) * int64(arg.Unit)), nil
	case *uint:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *uint8:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *uint16:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *uint32:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case *uint64:
		return parseTimestamp(int64(*v) * int64(arg.Unit)), nil
	case float32:
		return parseTimestamp(int64(float64(v) * float64(arg.Unit))), nil
	case float64:
		return parseTimestamp(int64((v) * float64(arg.Unit))), nil
	case *float32:
		return parseTimestamp(int64(float64(*v) * float64(arg.Unit))), nil
	case *float64:
		return parseTimestamp(int64((*v) * float64(arg.Unit))), nil
	case string:
		return time.Parse(arg.Format, v)
	case *string:
		return time.Parse(arg.Format, *v)
	case time.Time:
		return v, nil
	case *time.Time:
		return *v, nil
	default:
		return time.Time{}, fmt.Errorf("unable to cast %#v of type %T to Time", i, i)
	}
}

// parseTimestamp parse nanosecond time.
func parseTimestamp(v int64) time.Time {
	return time.Unix(v/int64(time.Second), v%int64(time.Second))
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

	switch v := reflect.ValueOf(i); v.Kind() {
	case reflect.Slice, reflect.Array:
		var slice []string
		for j := 0; j < v.Len(); j++ {
			s, err := ToStringE(v.Index(j).Interface())
			if err != nil {
				return nil, err
			}
			slice = append(slice, s)
		}
		return slice, nil
	}

	return nil, fmt.Errorf("unable to cast %#v of type %T to []string", i, i)
}
