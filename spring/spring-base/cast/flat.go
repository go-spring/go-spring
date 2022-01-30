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
	"strconv"
	"strings"
)

const rootKey = "$"

// FlatBytes 将任意字符切片解析成一维映射表，如 {"a":{"b":"c"}} 解析成 a.b=c 映射表。
func FlatBytes(data []byte) map[string]interface{} {
	result := make(map[string]interface{})
	flatBytes(rootKey, data, result)
	return result
}

func flatBytes(prefix string, data []byte, result map[string]interface{}) {
	switch trimData := bytes.TrimSpace(data); trimData[0] {
	case '{':
		var tempMap map[string]json.RawMessage
		if json.Unmarshal(trimData, &tempMap) != nil {
			result[prefix] = string(data)
			return
		}
		for k, v := range tempMap {
			tempBytes := v
			if tempBytes[0] == '"' {
				if s, err := strconv.Unquote(string(tempBytes)); err == nil {
					tempBytes = []byte(s)
				}
			}
			if prefix != rootKey {
				k = prefix + "." + k
			}
			if trimBytes := bytes.TrimSpace(tempBytes); trimBytes[0] == '"' {
				if s, err := strconv.Unquote(string(trimBytes)); err == nil {
					tempBytes = []byte(s)
					k += ".\"\""
				}
			}
			flatBytes(k, tempBytes, result)
		}
	case '[':
		var tempSlice []json.RawMessage
		if json.Unmarshal(trimData, &tempSlice) != nil {
			result[prefix] = string(data)
			return
		}
		for i, v := range tempSlice {
			k := fmt.Sprintf("[%d]", i)
			if prefix != rootKey {
				k = prefix + k
			}
			flatBytes(k, v, result)
		}
	default:
		var i interface{}
		if json.Unmarshal(data, &i) != nil {
			result[prefix] = string(data)
			return
		}
		s, ok := i.(string)
		if !ok {
			result[prefix] = i
			return
		}
		switch trimStr := strings.TrimSpace(s); trimStr[0] {
		case '{', '[':
			k := "\"\""
			if prefix != rootKey {
				k = prefix + "." + k
			}
			flatBytes(k, []byte(trimStr), result)
		case '"':
			var err error
			trimStr, err = strconv.Unquote(trimStr)
			if err != nil {
				result[prefix] = i
				return
			}
			k := "\"\""
			if prefix != rootKey {
				k = prefix + "." + k
			}
			flatBytes(k, []byte(trimStr), result)
		default:
			result[prefix] = i
		}
	}
}
