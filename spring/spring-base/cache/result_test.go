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

package cache_test

import (
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cache"
)

func TestValueResult(t *testing.T) {

	t.Run("", func(t *testing.T) {
		r := cache.NewValueResult(map[string]string{
			"a": "123",
		})
		b, err := r.JSON()
		assert.Nil(t, err)
		assert.Equal(t, b, `{"a":"123"}`)
		var m map[string]string
		err = r.Load(m)
		assert.Error(t, err, "value should be ptr and not nil")
		err = r.Load(&m)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]string{
			"a": "123",
		})
		var s []string
		err = r.Load(&s)
		assert.Error(t, err, "load type \\(\\[\\]string\\) but expect type \\(map\\[string\\]string\\)")
	})

	t.Run("", func(t *testing.T) {
		r := cache.NewValueResult([]string{
			"abc",
		})
		b, err := r.JSON()
		assert.Nil(t, err)
		assert.Equal(t, b, `["abc"]`)
		var s []string
		err = r.Load(&s)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{
			"abc",
		})
	})

	t.Run("", func(t *testing.T) {
		r := cache.NewValueResult(complex(1.0, 0.5))
		_, err := r.JSON()
		assert.Error(t, err, "json: unsupported type: complex128")
	})
}

func TestJSONResult(t *testing.T) {

	t.Run("", func(t *testing.T) {
		r := cache.NewJSONResult(`{"a":"123"}`)
		b, err := r.JSON()
		assert.Nil(t, err)
		assert.Equal(t, b, `{"a":"123"}`)
		var m map[string]string
		err = r.Load(m)
		assert.Error(t, err, "json: Unmarshal\\(non-pointer map\\[string\\]string\\)")
		err = r.Load(&m)
		assert.Nil(t, err)
		assert.Equal(t, m, map[string]string{
			"a": "123",
		})
		var s []string
		err = r.Load(&s)
		assert.Error(t, err, "json: cannot unmarshal object into Go value of type \\[\\]string")
	})

	t.Run("", func(t *testing.T) {
		r := cache.NewJSONResult(`["abc"]`)
		b, err := r.JSON()
		assert.Nil(t, err)
		assert.Equal(t, b, `["abc"]`)
		var s []string
		err = r.Load(&s)
		assert.Nil(t, err)
		assert.Equal(t, s, []string{
			"abc",
		})
	})
}
