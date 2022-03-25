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
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

const rootKey = "$"

var (
	numberType = reflect.TypeOf(json.Number(""))
)

func FlatJSON(data interface{}) map[string]string {
	result := make(map[string]string)
	switch v := data.(type) {
	case []byte:
		if !flatJSON(rootKey, v, result) {
			result[rootKey] = string(v)
		}
	case string:
		if !flatJSON(rootKey, []byte(v), result) {
			result[rootKey] = v
		}
	case [][]byte:
		for i, b := range v {
			k := rootKey + "[" + strconv.Itoa(i) + "]"
			if !flatJSON(k, b, result) {
				result[k] = string(b)
			}
		}
	case []string:
		for i, s := range v {
			k := rootKey + "[" + strconv.Itoa(i) + "]"
			if !flatJSON(k, []byte(s), result) {
				result[k] = s
			}
		}
	}
	return result
}

func flatJSON(prefix string, b []byte, result map[string]string) bool {
	var v interface{}
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&v); err != nil {
		return false
	}
	flatValue(prefix, v, result)
	return true
}

func flatValue(prefix string, v interface{}, result map[string]string) {
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		result[prefix] = "null"
		return
	}
	switch val.Kind() {
	case reflect.Map:
		if val.Len() == 0 {
			result[prefix] = "{}"
			return
		}
		iter := val.MapRange()
		for iter.Next() {
			key := ToString(iter.Key().Interface())
			key = prefix + "[" + key + "]"
			flatValue(key, iter.Value().Interface(), result)
		}
	case reflect.Array, reflect.Slice:
		if val.Len() == 0 {
			result[prefix] = "[]"
			return
		}
		for i := 0; i < val.Len(); i++ {
			key := prefix + "[" + strconv.Itoa(i) + "]"
			flatValue(key, val.Index(i).Interface(), result)
		}
	case reflect.Bool:
		result[prefix] = strconv.FormatBool(val.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result[prefix] = strconv.FormatInt(val.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result[prefix] = strconv.FormatUint(val.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		result[prefix] = strconv.FormatFloat(val.Float(), 'f', -1, 64)
	case reflect.String:
		if val.Type() == numberType {
			result[prefix] = val.String()
		} else {
			flatString(prefix, val.String(), result)
		}
	}
}

func flatString(prefix string, str string, result map[string]string) {
	trimBytes := bytes.TrimSpace([]byte(str))
	if len(trimBytes) == 0 {
		result[prefix] = strconv.Quote(str)
		return
	}
	switch trimBytes[0] {
	case '{', '[', '"':
		if !flatJSON(prefix+`[""]`, trimBytes, result) {
			result[prefix] = strconv.Quote(str)
		}
	default:
		result[prefix] = strconv.Quote(str)
	}
}

func FlatNode(node Node) map[string]string {
	result := map[string]string{}
	flatNodePrefix(rootKey, node, result)
	return result
}

func flatNodePrefix(prefix string, node Node, result map[string]string) {
	switch v := node.(type) {
	case *NilNode:
		result[prefix] = "<nil>"
	case *ValueNode:
		result[prefix] = v.Data
	case *MapNode:
		if len(v.Data) == 0 {
			result[prefix] = "{}"
			return
		}
		for key, data := range v.Data {
			if strings.Contains(key, ".") {
				key = "[" + key + "]"
			}
			flatNodePrefix(prefix+"."+key, data, result)
		}
	case *ArrayNode:
		if len(v.Data) == 0 {
			result[prefix] = "[]"
			return
		}
		for i, data := range v.Data {
			flatNodePrefix(prefix+"["+strconv.Itoa(i)+"]", data, result)
		}
	}
}
