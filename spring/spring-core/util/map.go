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

package util

import "fmt"

// FlatMap 将嵌套形式的 map 打平，map 必须是 map[string]interface{} 类型。
func FlatMap(m map[string]interface{}) map[string]interface{} {
	p := make(map[string]interface{})
	flatMap("", m, p)
	return p
}

func flatMap(prefix string, m map[string]interface{}, p map[string]interface{}) {
	for k, v := range m {
		flatValue(prefix+k, v, p)
	}
}

func flatSlice(prefix string, a []interface{}, p map[string]interface{}) {
	for i, v := range a {
		key := fmt.Sprintf("%s[%d]", prefix, i)
		flatValue(key, v, p)
	}
}

func flatValue(key string, v interface{}, p map[string]interface{}) {
	switch value := v.(type) {
	case map[string]interface{}:
		flatMap(key+".", value, p)
	case []interface{}:
		flatSlice(key, value, p)
	default:
		p[key] = v
	}
}
