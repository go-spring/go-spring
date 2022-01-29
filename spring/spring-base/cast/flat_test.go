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
	s := struct {
		A string `json:"a"`
		B struct {
			C string                 `json:"c"`
			D map[string]interface{} `json:"d"`
		} `json:"b"`
		M []struct {
			N string `json:"n"`
		} `json:"m"`
		Q []string      `json:"q"`
		R []interface{} `json:"r"`
	}{
		A: "a",
		B: struct {
			C string                 `json:"c"`
			D map[string]interface{} `json:"d"`
		}{
			C: "c",
			D: map[string]interface{}{"e": " \n\t\"{\\\"f\\\":\\\"g\\\",\\\"h\\\":\\\"i\\\"}\"\t\n "},
		},
		M: []struct {
			N string `json:"n"`
		}{
			{N: "\"n1\""},
			{N: "n2"},
		},
		Q: []string{"q1", "\"q2\"", " q3\t\n", "{\"a\":\"b\"}"},
		R: []interface{}{1, "2", 3},
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(string(b))
	m := cast.FlatBytes(b)
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		b, err = json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println(k, "=", string(b))
	}
	assert.Equal(t, m, map[string]interface{}{
		"a":            "a",
		"b.c":          "c",
		"b.d.e.\"\".f": "g",
		"b.d.e.\"\".h": "i",
		"m[0].n.\"\"":  "n1",
		"m[1].n":       "n2",
		"q[0]":         "q1",
		"q[1].\"\"":    "q2",
		"q[2]":         " q3\t\n",
		"q[3].\"\".a":  "b",
		"r[0]":         float64(1),
		"r[1]":         "2",
		"r[2]":         float64(3),
	})
}
