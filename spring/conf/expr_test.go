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

package conf_test

import (
	"errors"
	"testing"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

func TestExpr(t *testing.T) {
	conf.RegisterValidateFunc("checkInt", func(i int) (bool, error) {
		return i < 5, nil
	})
	conf.RegisterValidateFunc("checkIntWithErr", func(i int) (bool, error) {
		if i < 0 {
			return false, errors.New("negative number not allowed")
		}
		return i < 5, nil
	})

	t.Run("basic function validation", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"checkInt($)"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 4,
		}))
		err := conf.Bind(p, &v)
		assert.That(t, err).Nil()
		assert.That(t, 4).Equal(v.A)
	})

	t.Run("constant expression", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"$ < 10"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 5,
		}))
		err := conf.Bind(p, &v)
		assert.That(t, err).Nil()
		assert.That(t, 5).Equal(v.A)
	})

	t.Run("complex expression", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"$ >= 1 && $ <= 3"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 2,
		}))
		err := conf.Bind(p, &v)
		assert.That(t, err).Nil()
		assert.That(t, 2).Equal(v.A)
	})

	t.Run("validation failure", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"checkInt($)"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 14,
		}))
		err := conf.Bind(p, &v)
		assert.Error(t, err).Matches("expression evaluated to false")
	})

	t.Run("syntax error", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"checkInt(2$)"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 4,
		}))
		err := conf.Bind(p, &v)
		assert.Error(t, err).Matches("bad number syntax")
	})

	t.Run("return not bool", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"$+$"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 4,
		}))
		err := conf.Bind(p, &v)
		assert.Error(t, err).Matches("expression must return a boolean value")
	})

	t.Run("unregistered function", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:"unknownFunc($)"`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 5,
		}))
		err := conf.Bind(p, &v)
		assert.Error(t, err).Matches("invalid operation: cannot call nil")
	})

	t.Run("empty expression", func(t *testing.T) {
		var v struct {
			A int `value:"${a}" expr:""`
		}
		p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
			"a": 5,
		}))
		err := conf.Bind(p, &v)
		assert.That(t, err).Nil()
		assert.That(t, 5).Equal(v.A)
	})

		t.Run("validate function returns error", func(t *testing.T) {
			var v struct {
				A int `value:"${a}" expr:"checkIntWithErr($)"`
			}
			p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
				"a": -1,
			}))
			err := conf.Bind(p, &v)
			assert.Error(t, err).Matches("negative number not allowed")
		})
	}
