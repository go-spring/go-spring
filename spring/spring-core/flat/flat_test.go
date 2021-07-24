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

package flat_test

import (
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/flat"
)

func TestFlatMap(t *testing.T) {
	m := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "d",
			},
			"e": []interface{}{
				"f", "g",
			},
			"h": "i",
		},
		"j": []interface{}{
			"k", "l",
		},
		"m": "n",
	}
	ret := flat.Map(m)
	expect := map[string]interface{}{
		"a.b.c":  "d",
		"a.e[0]": "f",
		"a.e[1]": "g",
		"a.h":    "i",
		"j[0]":   "k",
		"j[1]":   "l",
		"m":      "n",
	}
	assert.Equal(t, ret, expect)
}
