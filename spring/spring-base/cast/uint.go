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

func UintPtr(s uint) *uint       { return &s }
func Uint8Ptr(s uint8) *uint8    { return &s }
func Uint16Ptr(s uint16) *uint16 { return &s }
func Uint32Ptr(s uint32) *uint32 { return &s }
func Uint64Ptr(s uint64) *uint64 { return &s }

// ToUint casts an interface{} to an uint.
// When type is clear, it is recommended to use standard library functions.
func ToUint(i interface{}) uint {
	v, _ := ToUint64E(i)
	return uint(v)
}

// ToUint8 casts an interface{} to an uint8.
// When type is clear, it is recommended to use standard library functions.
func ToUint8(i interface{}) uint8 {
	v, _ := ToUint64E(i)
	return uint8(v)
}

// ToUint16 casts an interface{} to an uint16.
// When type is clear, it is recommended to use standard library functions.
func ToUint16(i interface{}) uint16 {
	v, _ := ToUint64E(i)
	return uint16(v)
}

// ToUint32 casts an interface{} to an uint32.
// When type is clear, it is recommended to use standard library functions.
func ToUint32(i interface{}) uint32 {
	v, _ := ToUint64E(i)
	return uint32(v)
}

// ToUint64 casts an interface{} to an uint64.
// When type is clear, it is recommended to use standard library functions.
func ToUint64(i interface{}) uint64 {
	v, _ := ToUint64E(i)
	return v
}

// ToUint64E casts an interface{} to an uint64.
// When type is clear, it is recommended to use standard library functions.
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
	return 0, fmt.Errorf("unable to cast type %T to uint64", i)
}
