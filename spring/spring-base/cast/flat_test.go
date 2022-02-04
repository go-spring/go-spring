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

package cast_test

import (
	"encoding/json"
	"fmt"
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func TestFlat(t *testing.T) {
	str := `{
		"a": "a",
		"b": {
			"c": "c",
			"d": "{\"e\":\"e\",\"f\":\"{\\\"g\\\":\\\"}\\\",\\\"h\\\":\\\"{\\\\\\\"i\\\\\\\":\\\\\\\"{\\\\\\\"}\\\"}\"}"
		},
		"q": ["1", "2", "3"],
		"q": [1, "2", 3],
		"r": ["1", "{\"e\":\"e\",\"f\":\"{\\\"g\\\":\\\"]\\\",\\\"h\\\":\\\"{\\\\\\\"i\\\\\\\":\\\\\\\"[\\\\\\\"}\\\"}\"}", 3],
		"s": "{",
		"t": "[",
		"x": "{}",
		"y": "[]",
		"z": {
			"a": " ",
			"b": "\t",
			"c": "\n",
			"d": "\r\n",
			"e": "@\"\\x00\""
		}
	}`
	m := cast.Flat([]byte(str))
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(k, "=", string(b))
	}
	assert.Equal(t, m, map[string]string{
		"$.a":                         "a",
		"$.b.c":                       "c",
		"$.b.d.\"\".e":                "e",
		"$.b.d.\"\".f.\"\".g":         "}",
		"$.b.d.\"\".f.\"\".h.\"\".i":  "{",
		"$.q[0]":                      "1",
		"$.q[1]":                      "2",
		"$.q[2]":                      "3",
		"$.r[0]":                      "1",
		"$.r[1].\"\".e":               "e",
		"$.r[1].\"\".f.\"\".g":        "]",
		"$.r[1].\"\".f.\"\".h.\"\".i": "[",
		"$.r[2]":                      "3",
		"$.s":                         "{",
		"$.t":                         "[",
		"$.x":                         "{}",
		"$.y":                         "[]",
		"$.z.a":                       " ",
		"$.z.b":                       "\t",
		"$.z.c":                       "\n",
		"$.z.d":                       "\r\n",
		"$.z.e":                       "@\"\\x00\"",
	})
}

func TestFlatSlice(t *testing.T) {
	m := cast.FlatSlice([]string{
		"a", "[\"b\",\"c\"]", "{\"c\":\"d\",\"e\":[\"f\"]}",
	})
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(k, "=", string(b))
	}
	assert.Equal(t, m, map[string]string{
		"$[0]":      "a",
		"$[1][0]":   "b",
		"$[1][1]":   "c",
		"$[2].c":    "d",
		"$[2].e[0]": "f",
	})
}
