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

package json

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestRead(t *testing.T) {

	t.Run("invalid json format", func(t *testing.T) {
		_, err := Read([]byte(`{`))
		assert.Error(t, err).Matches("unexpected end of JSON input")
	})

	t.Run("basic data types", func(t *testing.T) {
		r, err := Read([]byte(`{
			"empty": "",
			"bool": false,
			"int": 3,
			"float": 3.0,
			"string": "hello",
			"date": "2018-02-17",
			"time": "2018-02-17T15:02:31+08:00"
		}`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"empty":  "",
			"bool":   false,
			"int":    float64(3),
			"float":  3.0,
			"string": "hello",
			"date":   "2018-02-17",
			"time":   "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("nested objects", func(t *testing.T) {
		r, err := Read([]byte(`{
			"user": {
				"name": "Alice",
				"age": 30
			},
			"active": true
		}`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"user": map[string]any{
				"name": "Alice",
				"age":  float64(30),
			},
			"active": true,
		})
	})

	t.Run("arrays", func(t *testing.T) {
		r, err := Read([]byte(`{
			"tags": ["go", "spring"],
			"numbers": [1, 2]
		}`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"tags":    []any{"go", "spring"},
			"numbers": []any{float64(1), float64(2)},
		})
	})

	t.Run("null values", func(t *testing.T) {
		r, err := Read([]byte(`{
			"value": null,
			"name": "test"
		}`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"value": nil,
			"name":  "test",
		})
	})

	t.Run("empty object", func(t *testing.T) {
		r, err := Read([]byte(`{}`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{})
	})
}
