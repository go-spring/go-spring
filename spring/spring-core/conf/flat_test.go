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

package conf_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf"
)

func TestFlatten(t *testing.T) {
	m := map[string]interface{}{
		"int": 123,
		"str": "abc",
		"arr": []interface{}{
			"abc",
			"def",
			map[string]interface{}{
				"a": "123",
				"b": "456",
			},
		},
		"map": map[string]interface{}{
			"a": "123",
			"b": "456",
			"arr": []string{
				"abc",
				"def",
			},
		},
	}
	expect := map[string]string{
		"int":        "123",
		"str":        "abc",
		"map.a":      "123",
		"map.b":      "456",
		"map.arr[0]": "abc",
		"map.arr[1]": "def",
		"arr[0]":     "abc",
		"arr[1]":     "def",
		"arr[2].a":   "123",
		"arr[2].b":   "456",
	}
	assert.Equal(t, conf.Flatten(m), expect)
}
