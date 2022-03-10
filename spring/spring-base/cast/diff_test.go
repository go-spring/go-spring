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
	"math"
	"sort"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func printDiff(t *testing.T, m map[string]cast.DiffItem) {
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
}

func TestDiffJSON(t *testing.T) {

	var testcases = []struct {
		a, b   string
		expect map[string]cast.DiffItem
		opts   []cast.DiffOption
	}{
		{
			a:      "3",
			b:      "3",
			expect: map[string]cast.DiffItem{},
		},
		{
			a: "3",
			b: "b",
			expect: map[string]cast.DiffItem{
				"$": {
					A: "3",
					B: "b",
				},
			},
		},
		{
			a:      "3",
			b:      "b",
			expect: map[string]cast.DiffItem{},
			opts: []cast.DiffOption{
				cast.Ignore("$"),
			},
		},
		{
			a:      "3.00",
			b:      "3.01",
			expect: map[string]cast.DiffItem{},
			opts: []cast.DiffOption{
				cast.Compare("$", func(a, b string) bool {
					na := cast.ToFloat64(a)
					nb := cast.ToFloat64(b)
					return math.Abs(na-nb) < 0.5
				}),
			},
		},
		{
			a:      `{"a":"b"}`,
			b:      `{"a":"b"}`,
			expect: map[string]cast.DiffItem{},
			opts:   []cast.DiffOption{},
		},
		{
			a:      `{"a":"3"}`,
			b:      `{"a":"b"}`,
			expect: map[string]cast.DiffItem{},
			opts: []cast.DiffOption{
				cast.Ignore("$.a"),
			},
		},
		{
			a:      `[3,4,5]`,
			b:      `[3,4,5]`,
			expect: map[string]cast.DiffItem{},
			opts:   []cast.DiffOption{},
		},
		{
			a: `[3,4,5]`,
			b: `[3,"4",5]`,
			expect: map[string]cast.DiffItem{
				"$[1]": {
					A: `4`,
					B: `"4"`,
				},
			},
			opts: []cast.DiffOption{},
		},
	}

	for _, c := range testcases {
		m, err := cast.DiffJSON(c.a, c.b, c.opts...)
		assert.Nil(t, err)
		printDiff(t, m)
		assert.Equal(t, m, c.expect)
	}
}
