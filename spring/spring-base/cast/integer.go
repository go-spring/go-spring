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
	"fmt"
	"strconv"
)

// ToInt casts an interface{} to an int. 在类型明确的情况下推荐使用标准库函数。
func ToInt(i interface{}) int {
	v, _ := ToInt64E(i)
	return int(v)
}

// ToInt8 casts an interface{} to an int8. 在类型明确的情况下推荐使用标准库函数。
func ToInt8(i interface{}) int8 {
	v, _ := ToInt64E(i)
	return int8(v)
}

// ToInt16 casts an interface{} to an int16. 在类型明确的情况下推荐使用标准库函数。
func ToInt16(i interface{}) int16 {
	v, _ := ToInt64E(i)
	return int16(v)
}

// ToInt32 casts an interface{} to an int32. 在类型明确的情况下推荐使用标准库函数。
func ToInt32(i interface{}) int32 {
	v, _ := ToInt64E(i)
	return int32(v)
}

// ToInt64 casts an interface{} to an int64. 在类型明确的情况下推荐使用标准库函数。
func ToInt64(i interface{}) int64 {
	v, _ := ToInt64E(i)
	return v
}

// ToInt64E casts an interface{} to an int64. 在类型明确的情况下推荐使用标准库函数。
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
		return strconv.ParseInt(s, 0, 0)
	case *string:
		return strconv.ParseInt(*s, 0, 0)
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

// ToUint casts an interface{} to an uint. 在类型明确的情况下推荐使用标准库函数。
func ToUint(i interface{}) uint {
	v, _ := ToUint64E(i)
	return uint(v)
}

// ToUint8 casts an interface{} to an uint8. 在类型明确的情况下推荐使用标准库函数。
func ToUint8(i interface{}) uint8 {
	v, _ := ToUint64E(i)
	return uint8(v)
}

// ToUint16 casts an interface{} to an uint16. 在类型明确的情况下推荐使用标准库函数。
func ToUint16(i interface{}) uint16 {
	v, _ := ToUint64E(i)
	return uint16(v)
}

// ToUint32 casts an interface{} to an uint32. 在类型明确的情况下推荐使用标准库函数。
func ToUint32(i interface{}) uint32 {
	v, _ := ToUint64E(i)
	return uint32(v)
}

// ToUint64 casts an interface{} to an uint64. 在类型明确的情况下推荐使用标准库函数。
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface{} to an uint64. 在类型明确的情况下推荐使用标准库函数。
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
		return strconv.ParseUint(s, 0, 0)
	case *string:
		return strconv.ParseUint(*s, 0, 0)
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
