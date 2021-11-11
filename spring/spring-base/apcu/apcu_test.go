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

package apcu_test

import (
	"context"
	"testing"

	"github.com/go-spring/spring-base/apcu"
	"github.com/go-spring/spring-base/assert"
)

func TestStore(t *testing.T) {
	ctx := context.Background()

	t.Run("int", func(t *testing.T) {

		var i int
		load, err := apcu.Load(ctx, "int", &i)
		assert.Nil(t, err)
		assert.False(t, load)

		apcu.Store(ctx, "int", 3)

		load, err = apcu.Load(ctx, "int", &i)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, i, 3)

		apcu.Delete(ctx, "int")
	})

	t.Run("string", func(t *testing.T) {

		var s string
		load, err := apcu.Load(ctx, "string", &s)
		assert.Nil(t, err)
		assert.False(t, load)

		apcu.Store(ctx, "string", "this is a simple string")

		load, err = apcu.Load(ctx, "string", &s)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, s, "this is a simple string")

		apcu.Delete(ctx, "string")
	})

	t.Run("json string", func(t *testing.T) {

		var s string
		load, err := apcu.Load(ctx, "string", &s)
		assert.Nil(t, err)
		assert.False(t, load)

		apcu.Store(ctx, "string", "\"this is a json string\"")

		load, err = apcu.Load(ctx, "string", &s)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, s, "this is a json string")

		apcu.Delete(ctx, "string")
	})

	type Resp struct {
		ErrNo  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
	}

	t.Run("json object", func(t *testing.T) {
		apcu.Store(ctx, "success", `{"errno":200,"errmsg":"OK"}`)

		var resp *Resp
		load, err := apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, &Resp{200, "OK"})

		load, err = apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, &Resp{200, "OK"})

		apcu.Delete(ctx, "success")
	})

	t.Run("map[string]interface{}", func(t *testing.T) {
		apcu.Store(ctx, "success", `{"errno":200,"errmsg":"OK"}`)

		var resp map[string]interface{}
		load, err := apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, map[string]interface{}{
			"errno": float64(200), "errmsg": "OK",
		})

		load, err = apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, map[string]interface{}{
			"errno": float64(200), "errmsg": "OK",
		})
	})
}
