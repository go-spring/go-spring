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
	"fmt"
	"strings"
)

const rootKey = "$"

// FlatJSON 将 JSON 字符串解析成一维映射表，如 {"a":{"b":"c"}} 解析为 {"$a.b":"c"}。
func FlatJSON(data interface{}) map[string]string {
	result := make(map[string]string)
	switch v := data.(type) {
	case []byte:
		if !flatJsonPrefix(rootKey, v, result) {
			result[rootKey] = string(v)
		}
	case string:
		if !flatJsonPrefix(rootKey, []byte(v), result) {
			result[rootKey] = v
		}
	case [][]byte:
		for i, b := range v {
			k := rootKey + fmt.Sprintf("[%d]", i)
			if !flatJsonPrefix(k, b, result) {
				result[k] = string(b)
			}
		}
	case []string:
		for i, s := range v {
			k := rootKey + fmt.Sprintf("[%d]", i)
			if !flatJsonPrefix(k, []byte(s), result) {
				result[k] = s
			}
		}
	}
	return result
}

// flatJsonPrefix JSON 可以是数字(整数或浮点数)、字符串("")、布尔值(true/false)、数组([])、
// 对象({})、空值(null)。
func flatJsonPrefix(prefix string, data []byte, result map[string]string) bool {
	switch tempData := bytes.TrimSpace(data); tempData[0] {
	case '{':
		var m map[string]json.RawMessage
		if json.Unmarshal(tempData, &m) != nil {
			return false
		}
		if len(m) == 0 {
			result[prefix] = "{}"
			return true
		}
		for k, v := range m {
			k = prefix + "." + k
			if !flatJsonPrefix(k, v, result) {
				result[k] = string(v)
			}
		}
		return true
	case '[':
		var m []json.RawMessage
		if json.Unmarshal(tempData, &m) != nil {
			return false
		}
		if len(m) == 0 {
			result[prefix] = "[]"
			return true
		}
		for i, v := range m {
			k := prefix + fmt.Sprintf("[%d]", i)
			if !flatJsonPrefix(k, v, result) {
				result[k] = string(v)
			}
		}
		return true
	default:
		var strTemp string
		if json.Unmarshal(data, &strTemp) != nil {
			result[prefix] = string(data)
			return true
		}
		strTemp = strings.TrimSpace(strTemp)
		if len(strTemp) == 0 {
			result[prefix] = string(data)
			return true
		}
		switch strTemp[0] {
		case '{', '[', '"':
			k := prefix + ".\"\""
			if !flatJsonPrefix(k, []byte(strTemp), result) {
				result[prefix] = string(data)
			}
			return true
		default:
			result[prefix] = string(data)
			return true
		}
	}
}
