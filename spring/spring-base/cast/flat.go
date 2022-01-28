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
	"encoding/json"
	"fmt"
	"strconv"
)

// Flat 将任意 json 字符串解析成扁平映射表，如 {"a":{"b":"c"}} 解析成 a.b=c 映射。
func Flat(data []byte) (map[string]interface{}, error) {
	return flat("", data)
}

func flat(key string, data []byte) (map[string]interface{}, error) {

	var (
		result    = make(map[string]interface{})
		tempMap   map[string]json.RawMessage
		tempSlice []json.RawMessage
	)

	// 下面分支表示 json 字符串是 struct 或者 map 结构。
	if json.Unmarshal(data, &tempMap) == nil {
		for k, v := range tempMap {
			b := v
			if b[0] == '"' { // 如果是字符串需要解引用一次。
				s, err := strconv.Unquote(string(b))
				if err == nil {
					b = []byte(s)
				}
			}
			ret, err := flat(k, b)
			if err != nil {
				return nil, err
			}
			for tempKey, tempVal := range ret {
				if key != "" {
					tempKey = key + "." + tempKey
				}
				result[tempKey] = tempVal
			}
		}
		return result, nil
	}

	// 下面分支表示 json 字符串是 slice 或者 array 结构。
	if json.Unmarshal(data, &tempSlice) == nil {
		for i, v := range tempSlice {
			b := v
			if b[0] == '"' { // 如果是字符串需要解引用一次。
				s, err := strconv.Unquote(string(b))
				if err == nil {
					b = []byte(s)
				}
			}
			ret, err := flat(fmt.Sprintf("[%d]", i), b)
			if err != nil {
				return nil, err
			}
			for tempKey, tempVal := range ret {
				if key != "" {
					tempKey = key + tempKey
				}
				result[tempKey] = tempVal
			}
		}
		return result, nil
	}

	if data[0] == '"' { // 执行到这里表明嵌套了 json 字符串。
		s, err := strconv.Unquote(string(data))
		if err == nil {
			var ret map[string]interface{}
			ret, err = flat("\"\"", []byte(s))
			if err != nil {
				return nil, err
			}
			for tempKey, tempVal := range ret {
				if key != "" {
					tempKey = key + "." + tempKey
				}
				result[tempKey] = tempVal
			}
			return result, nil
		}
	}

	var i interface{}
	if json.Unmarshal(data, &i) == nil {
		result[key] = i
		return result, nil
	}

	result[key] = string(data)
	return result, nil
}
