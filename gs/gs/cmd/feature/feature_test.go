/*
 * Copyright 2025 The Go-Spring Authors.
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

package feature

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestParseParams(t *testing.T) {
	spec := map[string]Param{
		"instances": {Type: "list", Default: []any{"default"}},
		"charset":   {Type: "string", Default: "utf8mb4"},
		"driver":    {Type: "string", Enum: []string{"mysql", "mysql8"}},
	}

	t.Run("bare flag yields defaults", func(t *testing.T) {
		got, err := ParseParams("", spec)
		assert.That(t, err).Nil()
		assert.That(t, got["instances"]).Equal("default")
		assert.That(t, got["charset"]).Equal("utf8mb4")
	})

	t.Run("value overrides default; list keeps commas", func(t *testing.T) {
		got, err := ParseParams("instances=order,user;charset=gbk", spec)
		assert.That(t, err).Nil()
		assert.That(t, got["instances"]).Equal("order,user")
		assert.That(t, got["charset"]).Equal("gbk")
	})

	t.Run("unknown key rejected", func(t *testing.T) {
		_, err := ParseParams("bogus=1", spec)
		assert.Error(t, err).Matches("unknown parameter")
	})

	t.Run("missing assignment rejected", func(t *testing.T) {
		_, err := ParseParams("instances", spec)
		assert.Error(t, err).Matches("malformed feature parameter")
	})

	t.Run("enum enforced on scalar", func(t *testing.T) {
		_, err := ParseParams("driver=postgres", spec)
		assert.Error(t, err).Matches("not in allowed set")
	})

	t.Run("enum enforced per list element", func(t *testing.T) {
		listSpec := map[string]Param{
			"backends": {Type: "list", Enum: []string{"a", "b"}},
		}
		_, err := ParseParams("backends=a,c", listSpec)
		assert.Error(t, err).Matches("not in allowed set")
		got, err := ParseParams("backends=a,b", listSpec)
		assert.That(t, err).Nil()
		assert.That(t, got["backends"]).Equal("a,b")
	})
}
