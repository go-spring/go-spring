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

func BoolPtr(s bool) *bool { return &s }

// ToBool casts an interface{} to a bool.
// When type is clear, it is recommended to use standard library functions.
func ToBool(i interface{}) bool {
	v, _ := ToBoolE(i)
	return v
}

// ToBoolE casts an interface{} to a bool.
// When type is clear, it is recommended to use standard library functions.
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
		return false, fmt.Errorf("unable to cast type (%T) to bool", i)
	}
}
