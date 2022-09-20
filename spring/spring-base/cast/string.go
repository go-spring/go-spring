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
	"html/template"
	"strconv"
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
