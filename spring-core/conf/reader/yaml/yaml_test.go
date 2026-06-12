/*
 * Copyright 2024 The Go-Spring Authors.
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

package yaml

import (
	"strings"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestRead(t *testing.T) {

	t.Run("invalid yaml format", func(t *testing.T) {
		_, err := Read([]byte(`{`))
		assert.Error(t, err).Matches("did not find expected node content")
	})

	t.Run("basic type", func(t *testing.T) {
		str := `
			empty: ""
			bool: false
			int: 3
			float: 3.0
			string1: "3"
			string2: hello
			date: 2018-02-17
			time: 2018-02-17T15:02:31+08:00
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := Read([]byte(str))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"empty":   "",
			"bool":    false,
			"int":     3,
			"float":   3.0,
			"string1": "3",
			"string2": "hello",
			"date":    "2018-02-17",
			"time":    "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("simple map", func(t *testing.T) {
		str := `
			map:
				bool: false
				int: 3
				float: 3.0
				string: hello
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := Read([]byte(str))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"map": map[any]any{
				"bool":   false,
				"float":  3.0,
				"int":    3,
				"string": "hello",
			},
		})
	})

	t.Run("array with struct", func(t *testing.T) {
		str := `
			array:
				-
					bool: false
					int: 3
					float: 3.0
					string: hello
				-
					bool: true
					int: 20
					float: 0.2
					string: hello
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := Read([]byte(str))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"array": []any{
				map[any]any{
					"bool":   false,
					"int":    3,
					"float":  3.0,
					"string": "hello",
				},
				map[any]any{
					"bool":   true,
					"int":    20,
					"float":  0.2,
					"string": "hello",
				},
			},
		})
	})

	t.Run("map with struct", func(t *testing.T) {
		str := `
			map:
				k1:
					bool: false
					int: 3
					float: 3.0
					string: hello
				k2:
					bool: true
					int: 20
					float: 0.2
					string: hello
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := Read([]byte(str))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"map": map[any]any{
				"k1": map[any]any{
					"bool":   false,
					"int":    3,
					"float":  3.0,
					"string": "hello",
				},
				"k2": map[any]any{
					"bool":   true,
					"int":    20,
					"float":  0.2,
					"string": "hello",
				},
			},
		})
	})

	t.Run("empty array & map", func(t *testing.T) {
		str := `
			array: []
			map: {}
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := Read([]byte(str))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"array": []any{},
			"map":   map[any]any{},
		})
	})
}
