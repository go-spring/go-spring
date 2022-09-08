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

package prop_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf/prop"
)

func TestRead(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {
		r, err := prop.Read([]byte(`
			bool=false
			int=3
			float=3.0
			string=hello
			date=2018-02-17
			time=2018-02-17T15:02:31+08:00
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"bool":   "false",
			"int":    "3",
			"float":  "3.0",
			"string": "hello",
			"date":   "2018-02-17",
			"time":   "2018-02-17T15:02:31+08:00",
		})
	})

	t.Run("map", func(t *testing.T) {
		r, err := prop.Read([]byte(`
			map.bool=false
			map.int=3
			map.float=3.0
			map.string=hello
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
			"map.bool":   "false",
			"map.int":    "3",
			"map.float":  "3.0",
			"map.string": "hello",
		})
	})

	t.Run("array struct", func(t *testing.T) {
		r, err := prop.Read([]byte(`
			array[0].bool=false
			array[0].int=3
			array[0].float=3.0
			array[0].string=hello
			array[1].bool=true
			array[1].int=20
			array[1].float=0.2
			array[1].string=hello
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
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

	t.Run("map struct", func(t *testing.T) {

		r, err := prop.Read([]byte(`
			map.k1.bool=false
			map.k1.int=3
			map.k1.float=3.0
			map.k1.string=hello
			map.k2.bool=true
			map.k2.int=20
			map.k2.float=0.2
			map.k2.string=hello
		`))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, r, map[string]interface{}{
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
}
