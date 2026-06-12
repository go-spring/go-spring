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
)

func TestExplain(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := Explain(nil, "%s", "test error")
		expected := "test error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
	})

	t.Run("no format args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Explain(originalErr, "static message")
		expected := "static message: original error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
		if !errors.Is(err, originalErr) {
			t.Errorf("expected error to wrap %q, but it did not", originalErr)
		}
	})

	t.Run("with formatted message and args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Explain(originalErr, "error %s %d", "message", 42)
		expected := "error message 42: original error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
		if !errors.Is(err, originalErr) {
			t.Errorf("expected error to wrap %q, but it did not", originalErr)
		}
	})

	t.Run("multiple nested errors", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := Explain(baseErr, "level 1")
		finalErr := Explain(wrappedErr, "level 2")
		expected := "level 2: level 1: base error"
		if finalErr.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, finalErr.Error())
		}
		if !errors.Is(finalErr, baseErr) {
			t.Errorf("expected error to wrap %q, but it did not", baseErr)
		}
		if !errors.Is(wrappedErr, errors.Unwrap(finalErr)) {
			t.Errorf("expected unwrapped error to be %q, but got %q", wrappedErr, errors.Unwrap(finalErr))
		}
	})
}

func TestStack(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		err := Stack(nil, "%s", "wrapped error")
		expected := "wrapped error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
	})

	t.Run("no format args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Stack(originalErr, "static wrapper")
		expected := "static wrapper >> original error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
		if !errors.Is(err, originalErr) {
			t.Errorf("expected error to wrap %q, but it did not", originalErr)
		}
	})

	t.Run("with formatted message and args", func(t *testing.T) {
		originalErr := errors.New("original error")
		err := Stack(originalErr, "wrapper %s %d", "text", 123)
		expected := "wrapper text 123 >> original error"
		if err.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, err.Error())
		}
		if !errors.Is(err, originalErr) {
			t.Errorf("expected error to wrap %q, but it did not", originalErr)
		}
	})

	t.Run("multiple nested errors", func(t *testing.T) {
		baseErr := errors.New("base error")
		wrappedErr := Stack(baseErr, "layer 1")
		finalErr := Stack(wrappedErr, "layer 2")
		expected := "layer 2 >> layer 1 >> base error"
		if finalErr.Error() != expected {
			t.Errorf("expected error %q, but got %q", expected, finalErr.Error())
		}
		if !errors.Is(finalErr, baseErr) {
			t.Errorf("expected error to wrap %q, but it did not", baseErr)
		}
		if !errors.Is(wrappedErr, errors.Unwrap(finalErr)) {
			t.Errorf("expected unwrapped error to be %q, but got %q", wrappedErr, errors.Unwrap(finalErr))
		}
	})
}
