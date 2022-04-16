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
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/conf"
)

func TestProperties_ReadToml(t *testing.T) {

	t.Run("basic type", func(t *testing.T) {

		data := []struct {
			key  string
			str  string
			val  interface{}
			kind reflect.Kind
		}{
			{"bool", "bool=false", "false", reflect.Bool},
			{"int", "int=3", "3", reflect.Int},
			{"float", "float=3.0", "3", reflect.Float64},
			{"string", "string=\"3\"", "3", reflect.String},
			{"string", "string=\"hello\"", "hello", reflect.String},
			{"date", "date=\"2018-02-17\"", "2018-02-17", reflect.String},
			{"time", "time=\"2018-02-17T15:02:31+08:00\"", "2018-02-17T15:02:31+08:00", reflect.String},
		}

		for _, d := range data {
			p, _ := conf.Bytes([]byte(d.str), ".toml")
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map", func(t *testing.T) {

		str := `
          [map]
          bool=false
          int=3
          float=3.0
          string="hello"
        `

		data := map[string]interface{}{
			"map.bool":   "false",
			"map.float":  "3",
			"map.int":    "3",
			"map.string": "hello",
		}

		p, _ := conf.Bytes([]byte(str), ".toml")
		for k, expect := range data {
			v := p.Get(k)
			assert.Equal(t, v, expect)
		}
	})

	t.Run("array struct", func(t *testing.T) {

		str := `
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
        `

		p, _ := conf.Bytes([]byte(str), ".toml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"array[0].bool", "false", reflect.Bool},
			{"array[0].int", "3", reflect.Int},
			{"array[0].float", "3", reflect.Float64},
			{"array[0].string", "hello", reflect.String},
			{"array[1].bool", "true", reflect.Bool},
			{"array[1].int", "20", reflect.Int},
			{"array[1].float", "0.2", reflect.Float64},
			{"array[1].string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})

	t.Run("map struct", func(t *testing.T) {

		str := `
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
        `

		p, _ := conf.Bytes([]byte(str), ".toml")

		data := []struct {
			key  string
			val  interface{}
			kind reflect.Kind
		}{
			{"map.k1.bool", "false", reflect.Bool},
			{"map.k1.int", "3", reflect.Int},
			{"map.k1.float", "3", reflect.Float64},
			{"map.k1.string", "hello", reflect.String},
			{"map.k2.bool", "true", reflect.Bool},
			{"map.k2.int", "20", reflect.Int},
			{"map.k2.float", "0.2", reflect.Float64},
			{"map.k2.string", "hello", reflect.String},
		}

		for _, d := range data {
			v := p.Get(d.key)
			assert.Equal(t, v, d.val)
		}
	})
}
