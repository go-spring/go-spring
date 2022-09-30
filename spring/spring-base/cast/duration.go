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
	"time"
)

// ToDuration casts an interface{} to a time.Duration.
// When type is clear, it is recommended to use standard library functions.
func ToDuration(i interface{}, unit ...time.Duration) time.Duration {
	v, _ := ToDurationE(i, unit...)
	return v
}

// ToDurationE casts an interface{} to a time.Duration.
// When type is clear, it is recommended to use standard library functions.
func ToDurationE(i interface{}, unit ...time.Duration) (time.Duration, error) {
	base := int64(time.Nanosecond)
	if len(unit) > 0 {
		base = int64(unit[0])
	}
	switch s := i.(type) {
	case nil:
		return 0, nil
	case int:
		return time.Duration(int64(s) * base), nil
	case int8:
		return time.Duration(int64(s) * base), nil
	case int16:
		return time.Duration(int64(s) * base), nil
	case int32:
		return time.Duration(int64(s) * base), nil
	case int64:
		return time.Duration(int64(s) * base), nil
	case *int:
		return time.Duration(int64(*s) * base), nil
	case *int8:
		return time.Duration(int64(*s) * base), nil
	case *int16:
		return time.Duration(int64(*s) * base), nil
	case *int32:
		return time.Duration(int64(*s) * base), nil
	case *int64:
		return time.Duration(int64(*s) * base), nil
	case uint:
		return time.Duration(int64(s) * base), nil
	case uint8:
		return time.Duration(int64(s) * base), nil
	case uint16:
		return time.Duration(int64(s) * base), nil
	case uint32:
		return time.Duration(int64(s) * base), nil
	case uint64:
		return time.Duration(int64(s) * base), nil
	case *uint:
		return time.Duration(int64(*s) * base), nil
	case *uint8:
		return time.Duration(int64(*s) * base), nil
	case *uint16:
		return time.Duration(int64(*s) * base), nil
	case *uint32:
		return time.Duration(int64(*s) * base), nil
	case *uint64:
		return time.Duration(int64(*s) * base), nil
	case float32:
		return time.Duration(float64(s) * float64(base)), nil
	case float64:
		return time.Duration(s * float64(base)), nil
	case *float32:
		return time.Duration(float64(*s) * float64(base)), nil
	case *float64:
		return time.Duration((*s) * float64(base)), nil
	case string:
		return time.ParseDuration(s)
	case *string:
		return time.ParseDuration(*s)
	case time.Duration:
		return s, nil
	default:
		return 0, fmt.Errorf("unable to cast type %T to time.Duration", i)
	}
}
