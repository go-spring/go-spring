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

// ToFloat32 casts an interface{} to a float32. 在类型明确的情况下推荐使用标准库函数。
func ToFloat32(i interface{}) float32 {
	v, _ := ToFloat64E(i)
	return float32(v)
}

// ToFloat64 casts an interface{} to a float64. 在类型明确的情况下推荐使用标准库函数。
func ToFloat64(i interface{}) float64 {
	v, _ := ToFloat64E(i)
	return v
}

// ToFloat64E casts an interface{} to a float64. 在类型明确的情况下推荐使用标准库函数。
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
