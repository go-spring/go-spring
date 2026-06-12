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

package prop

import (
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestRead(t *testing.T) {

	t.Run("invalid properties format", func(t *testing.T) {
		_, err := Read([]byte(`=1`))
		assert.Error(t, err).Matches(`properties: Line 1: "1"`)
	})

	t.Run("basic type", func(t *testing.T) {
		r, err := Read([]byte(`
			empty=
			bool=false
			int=3
			float=3.0
			string=hello
			date=2018-02-17
			time=2018-02-17T15:02:31+08:00
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"empty":  "",
			"bool":   "false",
			"int":    "3",
			"float":  "3.0",
			"string": "hello",
			"date":   "2018-02-17",
			"time":   "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("simple map", func(t *testing.T) {
		r, err := Read([]byte(`
			map.bool=false
			map.int=3
			map.float=3.0
			map.string=hello
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"map.bool":   "false",
			"map.int":    "3",
			"map.float":  "3.0",
			"map.string": "hello",
		})
	})

	t.Run("array with struct", func(t *testing.T) {
		r, err := Read([]byte(`
			array[0].bool=false
			array[0].int=3
			array[0].float=3.0
			array[0].string=hello
			array[1].bool=true
			array[1].int=20
			array[1].float=0.2
			array[1].string=hello
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"array[0].bool":   "false",
			"array[0].int":    "3",
			"array[0].float":  "3.0",
			"array[0].string": "hello",
			"array[1].bool":   "true",
			"array[1].int":    "20",
			"array[1].float":  "0.2",
			"array[1].string": "hello",
		})
	})

	t.Run("map with struct", func(t *testing.T) {
		r, err := Read([]byte(`
			map.k1.bool=false
			map.k1.int=3
			map.k1.float=3.0
			map.k1.string=hello
			map.k2.bool=true
			map.k2.int=20
			map.k2.float=0.2
			map.k2.string=hello
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"map.k1.bool":   "false",
			"map.k1.int":    "3",
			"map.k1.float":  "3.0",
			"map.k1.string": "hello",
			"map.k2.bool":   "true",
			"map.k2.int":    "20",
			"map.k2.float":  "0.2",
			"map.k2.string": "hello",
		})
	})

	t.Run("escape sequences", func(t *testing.T) {
		r, err := Read([]byte(`
			key1=value\nwith\nnewlines
			key2=value\twith\ttabs
			key3=unicode\u0041
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"key1": "value\nwith\nnewlines",
			"key2": "value\twith\ttabs",
			"key3": "unicodeA",
		})
	})

	t.Run("special characters", func(t *testing.T) {
		r, err := Read([]byte(`
			key.with.dots=value
			key-with-dashes=value
			unicode_key=值
		`))
		assert.That(t, err).Nil()
		assert.That(t, r).Equal(map[string]any{
			"key.with.dots":   "value",
			"key-with-dashes": "value",
			"unicode_key":     "值",
		})
	})
}
