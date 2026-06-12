/*
 * Copyright 2025 The Go-Spring Authors.
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

package flatten

import (
	"fmt"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestFlatten(t *testing.T) {
	testChan := make(chan int)
	testFunc := func() {}
	tests := []struct {
		name     string
		input    map[string]any
		expected map[string]string
	}{
		{
			name: "basic types",
			input: map[string]any{
				"int": 123,
				"str": "abc",
			},
			expected: map[string]string{
				"int": "123",
				"str": "abc",
			},
		},
		{
			name: "complex nested structures",
			input: map[string]any{
				"arr": []any{
					"abc",
					"def",
					map[string]any{
						"a": "123",
						"b": "456",
					},
					nil,
					([]any)(nil),
					(map[string]string)(nil),
					[]any{},
					map[string]string{},
				},
				"map": map[string]any{
					"a": "123",
					"b": "456",
					"arr": []string{
						"abc",
						"def",
					},
					"nil":       nil,
					"nil_arr":   []any(nil),
					"nil_map":   map[string]string(nil),
					"empty_arr": []any{},
					"empty_map": map[string]string{},
				},
				"nil":       nil,
				"nil_arr":   []any(nil),
				"nil_map":   map[string]string(nil),
				"empty_arr": []any{},
				"empty_map": map[string]string{},
			},
			expected: map[string]string{
				"nil":           "<nil>",
				"nil_arr":       "<nil>",
				"nil_map":       "<nil>",
				"empty_arr":     "[]",
				"empty_map":     "{}",
				"map.a":         "123",
				"map.b":         "456",
				"map.arr[0]":    "abc",
				"map.arr[1]":    "def",
				"map.empty_arr": "[]",
				"map.empty_map": "{}",
				"map.nil":       "<nil>",
				"map.nil_arr":   "<nil>",
				"map.nil_map":   "<nil>",
				"arr[0]":        "abc",
				"arr[1]":        "def",
				"arr[2].a":      "123",
				"arr[2].b":      "456",
				"arr[3]":        "<nil>",
				"arr[4]":        "<nil>",
				"arr[5]":        "<nil>",
				"arr[6]":        "[]",
				"arr[7]":        "{}",
			},
		},
		{
			name: "different value types",
			input: map[string]any{
				"bool":    true,
				"int":     42,
				"float":   3.14,
				"string":  "text",
				"complex": 1 + 2i,
			},
			expected: map[string]string{
				"bool":    "true",
				"int":     "42",
				"float":   "3.14",
				"string":  "text",
				"complex": "(1+2i)",
			},
		},
		{
			name: "deeply nested structures",
			input: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": map[string]any{
							"value": "deep",
						},
					},
				},
			},
			expected: map[string]string{
				"level1.level2.level3.value": "deep",
			},
		},
		{
			name: "chan and func values",
			input: map[string]any{
				"chan": testChan,
				"func": testFunc,
			},
			expected: map[string]string{
				"chan": fmt.Sprintf("%p", testChan),
				"func": fmt.Sprintf("%p", testFunc),
			},
		},
		{
			name: "interface with nil value",
			input: map[string]any{
				"iface": []any{any(nil)},
			},
			expected: map[string]string{
				"iface[0]": "<nil>",
			},
		},
		{
			name:     "empty input",
			input:    map[string]any{},
			expected: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Flatten(tt.input)
			assert.That(t, result).Equal(tt.expected)
		})
	}
}
