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

package conf

import (
	"strconv"
)

const rootKey = "$"

func Flat(node Node) map[string]string {
	result := map[string]string{}
	flatPrefix(rootKey, node, result)
	return result
}

func flatPrefix(prefix string, node Node, result map[string]string) {
	switch v := node.(type) {
	case *MapNode:
		if len(v.Data) == 0 {
			result[prefix] = "{}"
			return
		}
		for key, data := range v.Data {
			flatPrefix(prefix+"."+key, data, result)
		}
	case *ArrayNode:
		if len(v.Data) == 0 {
			result[prefix] = "[]"
			return
		}
		for i, data := range v.Data {
			flatPrefix(prefix+"["+strconv.Itoa(i)+"]", data, result)
		}
	case *ValueNode:
		result[prefix] = v.Data
	case *NilNode:
		result[prefix] = "<nil>"
	}
}
