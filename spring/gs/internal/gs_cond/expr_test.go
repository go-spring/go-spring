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

package gs_cond

import (
	"strconv"
	"strings"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestEvalExpr(t *testing.T) {
	t.Run("doesn't return bool", func(t *testing.T) {
		_, err := EvalExpr("$", "3")
		assert.Error(t, err).Matches("expression must return a boolean value")
	})

	t.Run("simple integer comparison", func(t *testing.T) {
		ok, err := EvalExpr("int($)==3", "3")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("custom function", func(t *testing.T) {
		RegisterExpressFunc("equal", func(s string, i int) bool {
			return s == strconv.Itoa(i)
		})
		ok, err := EvalExpr("equal($,9)", "9")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("complex boolean expression", func(t *testing.T) {
		ok, err := EvalExpr("int($)>0 && int($)<10", "5")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("boundary value testing", func(t *testing.T) {
		ok, err := EvalExpr("int($)==0", "0")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("string operations", func(t *testing.T) {
		RegisterExpressFunc("string_contains", func(s, substr string) bool {
			return len(s) >= len(substr) && strings.Contains(s, substr)
		})
		ok, err := EvalExpr("string_contains($, \"test\")", "this is a test")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})

	t.Run("unregistered function call", func(t *testing.T) {
		_, err := EvalExpr("unknownFunc($)", "3")
		assert.That(t, err).NotNil()
	})

	t.Run("empty string input", func(t *testing.T) {
		ok, err := EvalExpr("len($)==0", "")
		assert.That(t, err).Nil()
		assert.That(t, ok).True()
	})
}
