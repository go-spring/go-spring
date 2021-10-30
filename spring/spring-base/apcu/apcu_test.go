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

	type Resp struct {
		ErrNo  int    `json:"errno"`
		ErrMsg string `json:"errmsg"`
	}

	t.Run("", func(t *testing.T) {
		apcu.Store("success", `{"errno":200,"errmsg":"OK"}`)

		var resp *Resp
		load, err := apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, &Resp{200, "OK"})

		load, err = apcu.Load(context.TODO(), "success", &resp)
		assert.Nil(t, err)
		assert.True(t, load)
		assert.Equal(t, resp, &Resp{200, "OK"})
	})

	apcu.Delete("success")

	t.Run("", func(t *testing.T) {
		apcu.Store("success", `{"errno":200,"errmsg":"OK"}`)

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
