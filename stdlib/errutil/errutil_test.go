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

package errutil

import (
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestExplain(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := Explain(nil, "%s", "test error")
		assert.Error(t, err).String("test error")
	})

	t.Run("no format args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Explain(originalErr, "static message")
		assert.Error(t, err).String("static message: original error")
		assert.Error(t, err).Is(originalErr)
	})

	t.Run("with formatted message and args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Explain(originalErr, "error %s %d", "message", 42)
		assert.Error(t, err).String("error message 42: original error")
		assert.Error(t, err).Is(originalErr)
	})

	t.Run("multiple nested errors", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := Explain(baseErr, "level 1")
		finalErr := Explain(wrappedErr, "level 2")
		assert.Error(t, finalErr).String("level 2: level 1: base error")
		assert.Error(t, finalErr).Is(baseErr)
		assert.Error(t, wrappedErr).Is(errors.Unwrap(finalErr))
	})
}

func TestStack(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := Stack(nil, "%s", "wrapped error")
		assert.Error(t, err).String("wrapped error")
	})

	t.Run("no format args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Stack(originalErr, "static wrapper")
		assert.Error(t, err).String("static wrapper >> original error")
		assert.Error(t, err).Is(originalErr)
	})

	t.Run("with formatted message and args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Stack(originalErr, "wrapper %s %d", "text", 123)
		assert.Error(t, err).String("wrapper text 123 >> original error")
		assert.Error(t, err).Is(originalErr)
	})

	t.Run("multiple nested errors", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := Stack(baseErr, "layer 1")
		finalErr := Stack(wrappedErr, "layer 2")
		assert.Error(t, finalErr).String("layer 2 >> layer 1 >> base error")
		assert.Error(t, finalErr).Is(baseErr)
		assert.Error(t, wrappedErr).Is(errors.Unwrap(finalErr))
	})
}
