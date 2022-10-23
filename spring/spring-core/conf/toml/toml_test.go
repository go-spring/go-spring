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

package toml_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf/toml"
)

func TestRead(t *testing.T) {

	t.Run("error", func(t *testing.T) {
		_, err := toml.Read([]byte(`
			string=abc
		`))
		assert.NotNil(t, err)
	})

	t.Run("basic type", func(t *testing.T) {
		r, err := toml.Read([]byte(`
			bool=false
			int=3
			float=3.0
			string1="3"
			string2="hello"
			date="2018-02-17"
			time="2018-02-17T15:02:31+08:00"
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"bool":    false,
			"int":     int64(3),
			"float":   3.0,
			"string1": "3",
			"string2": "hello",
			"date":    "2018-02-17",
			"time":    "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("map", func(t *testing.T) {
		r, err := toml.Read([]byte(`
			[map]
			bool=false
			int=3
			float=3.0
			string="hello"
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"map": map[string]interface{}{
				"bool":   false,
				"float":  3.0,
				"int":    int64(3),
				"string": "hello",
			},
		})
	})

	t.Run("array struct", func(t *testing.T) {
		r, err := toml.Read([]byte(`
			[[array]]
			bool=false
			int=3
			float=3.0
			string="hello"
			
			[[array]]
			bool=true
			int=20
			float=0.2
			string="hello"
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"array": []interface{}{
				map[string]interface{}{
					"bool":   false,
					"int":    int64(3),
					"float":  3.0,
					"string": "hello",
				},
				map[string]interface{}{
					"bool":   true,
					"int":    int64(20),
					"float":  0.2,
					"string": "hello",
				},
			},
		})
	})

	t.Run("map struct", func(t *testing.T) {
		r, err := toml.Read([]byte(`
			[map.k1]
			bool=false
			int=3
			float=3.0
			string="hello"
			
			[map.k2]
			bool=true
			int=20
			float=0.2
			string="hello"
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"map": map[string]interface{}{
				"k1": map[string]interface{}{
					"bool":   false,
					"int":    int64(3),
					"float":  3.0,
					"string": "hello",
				},
				"k2": map[string]interface{}{
					"bool":   true,
					"int":    int64(20),
					"float":  0.2,
					"string": "hello",
				},
			},
		})
	})
}
