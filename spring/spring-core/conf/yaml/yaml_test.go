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

package yaml_test

import (
	"strings"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf/yaml"
)

func TestRead(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {
		str := `
			bool: false
			int: 3
			float: 3.0
			string1: "3"
			string2: hello
			date: 2018-02-17
			time: 2018-02-17T15:02:31+08:00
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := yaml.Read([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"bool":    false,
			"int":     3,
			"float":   3.0,
			"string1": "3",
			"string2": "hello",
			"date":    "2018-02-17",
			"time":    "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("map", func(t *testing.T) {
		str := `
			map:
				bool: false
				int: 3
				float: 3.0
				string: hello
		`
		str = strings.ReplaceAll(str, "\t", "  ")
		r, err := yaml.Read([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"map": map[interface{}]interface{}{
				"bool":   false,
				"float":  3.0,
				"int":    3,
				"string": "hello",
			},
		})
	})

	t.Run("array struct", func(t *testing.T) {
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
		r, err := yaml.Read([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"array": []interface{}{
				map[interface{}]interface{}{
					"bool":   false,
					"int":    3,
					"float":  3.0,
					"string": "hello",
				},
				map[interface{}]interface{}{
					"bool":   true,
					"int":    20,
					"float":  0.2,
					"string": "hello",
				},
			},
		})
	})

	t.Run("map struct", func(t *testing.T) {
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
		r, err := yaml.Read([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"map": map[interface{}]interface{}{
				"k1": map[interface{}]interface{}{
					"bool":   false,
					"int":    3,
					"float":  3.0,
					"string": "hello",
				},
				"k2": map[interface{}]interface{}{
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
		r, err := yaml.Read([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"array": []interface{}{},
			"map":   map[interface{}]interface{}{},
		})
	})
}
