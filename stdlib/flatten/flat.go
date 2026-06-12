/*
 * Copyright 2025 The Go-Spring Authors.
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

package flatten

import (
	"fmt"
	"reflect"
	"strconv"
)

// Flatten flattens a nested map[string]any into a map[string]string.
//
// This function is intended for data produced by encoding/json.Unmarshal,
// where values are limited to the following kinds:
//   - map[string]any
//   - []any
//   - primitive JSON types (bool, number, string, nil)
//
// Structs, custom types, and non-string map keys are explicitly out of scope.
//
// Flattening rules:
//
//   - Nested maps are expanded using dot notation:
//     {"a": {"b": 1}} -> "a.b" = "1"
//
//   - Slices (and arrays, although arrays do not originate from json.Unmarshal)
//     are expanded using index notation:
//     {"a": [1, 2]} -> "a[0]" = "1", "a[1]" = "2"
//
//   - Nil values (both untyped nil and typed nil) are represented as "<nil>".
//
//   - Empty (zero-length but non-nil) maps are represented as "{}".
//
//   - Empty (zero-length but non-nil) slices are represented as "[]".
//
//   - Primitive values are converted to strings using deterministic,
//     Go-native formatting (strconv).
//
// The resulting map is intended for display-oriented use cases such as
// logging, diffing, diagnostics, or inspection. The output is not reversible
// and must not be treated as a lossless serialization format.
func Flatten(m map[string]any) map[string]string {
	result := make(map[string]string)
	for key, val := range m {
		flattenValue(key, val, result)
	}
	return result
}

// flattenValue recursively expands v into result under the given key.
// Composite values are traversed depth-first, producing fully-qualified
// flattened keys.
func flattenValue(key string, val any, result map[string]string) {
	if val == nil { // untyped nil
		result[key] = "<nil>"
		return
	}
	switch v := reflect.ValueOf(val); v.Kind() {
	case reflect.Map:
		if v.IsNil() { // typed nil map
			result[key] = "<nil>"
			return
		}
		if v.Len() == 0 { // empty map
			result[key] = "{}"
			return
		}
		iter := v.MapRange()
		for iter.Next() {
			mapKey := toString(iter.Key())
			mapValue := iter.Value().Interface()
			flattenValue(key+"."+mapKey, mapValue, result)
		}
	case reflect.Slice:
		if v.IsNil() { // typed nil slice
			result[key] = "<nil>"
			return
		}
		if v.Len() == 0 { // empty slice
			result[key] = "[]"
			return
		}
		for i := range v.Len() {
			subKey := fmt.Sprintf("%s[%d]", key, i)
			subValue := v.Index(i).Interface()
			flattenValue(subKey, subValue, result)
		}
	case reflect.Interface, reflect.Pointer:
		if v.IsNil() { // typed nil interface or pointer
			result[key] = "<nil>"
			return
		}
		flattenValue(key, v.Elem().Interface(), result)
	default:
		result[key] = toString(v)
	}
}

// toString converts a reflect.Value representing a primitive JSON value
// into its string form. It intentionally supports only basic kinds and
// falls back to fmt.Sprintf for completeness.
func toString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.Bool:
		return strconv.FormatBool(v.Bool())
	case reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64:
		return strconv.FormatUint(v.Uint(), 10)
	case reflect.Float32,
		reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'f', -1, 64)
	case reflect.String:
		return v.String()
	default:
		return fmt.Sprintf("%v", v.Interface())
	}
}
