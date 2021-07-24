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

package flat

import (
	"fmt"
)

// Struct 将结构体的公开字段打平。
func Struct(v interface{}) map[string]interface{} {
	return make(map[string]interface{})
}

// Map 将嵌套形式的 map 打平，map 必须是 map[string]interface{} 类型。
func Map(m map[string]interface{}) map[string]interface{} {
	p := make(map[string]interface{})
	flatMap("", m, p)
	return p
}

func flatValue(key string, v interface{}, out map[string]interface{}) {
	switch value := v.(type) {
	case map[string]interface{}:
		flatMap(key+".", value, out)
	case []interface{}:
		flatSlice(key, value, out)
	default:
		out[key] = v
	}
}

func flatSlice(prefix string, arr []interface{}, out map[string]interface{}) {
	for i, v := range arr {
		key := fmt.Sprintf("%s[%d]", prefix, i)
		flatValue(key, v, out)
	}
}

func flatMap(prefix string, m map[string]interface{}, out map[string]interface{}) {
	for k, v := range m {
		flatValue(prefix+k, v, out)
	}
}
