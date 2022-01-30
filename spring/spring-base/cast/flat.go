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

// Flat 将 json 字符串解析成一维映射表，如 {"a":{"b":"c"}} 解析成 a.b=c 映射表。
func Flat(data []byte) map[string]interface{} {
	result := make(map[string]interface{})
	if !flat(rootKey, data, result) {
		result[rootKey] = string(data)
	}
	return result
}

func flat(prefix string, data []byte, result map[string]interface{}) bool {
	switch trimData := bytes.TrimSpace(data); trimData[0] {
	case '{':
		var m map[string]json.RawMessage
		if json.Unmarshal(trimData, &m) != nil {
			return false
		}
		for k, v := range m {
			if prefix != rootKey {
				k = prefix + "." + k
			}
			if !flat(k, v, result) {
				result[k] = string(v)
			}
		}
		return true
	case '[':
		var s []json.RawMessage
		if json.Unmarshal(trimData, &s) != nil {
			return false
		}
		for i, v := range s {
			k := fmt.Sprintf("[%d]", i)
			if prefix != rootKey {
				k = prefix + k
			}
			if !flat(k, v, result) {
				result[k] = string(v)
			}
		}
		return true
	default:
		var i interface{}
		if json.Unmarshal(data, &i) != nil {
			return false
		}
		s, ok := i.(string)
		if !ok {
			result[prefix] = i
			return true
		}
		switch trimStr := strings.TrimSpace(s); trimStr[0] {
		case '{', '[', '"':
			k := "\"\""
			if prefix != rootKey {
				k = prefix + "." + k
			}
			if !flat(k, []byte(trimStr), result) {
				result[prefix] = s
			}
			return true
		default:
			result[prefix] = s
			return true
		}
	}
}
