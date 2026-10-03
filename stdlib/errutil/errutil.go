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

// Package errutil provides lightweight utilities for three error-related
// concerns:
//
//  1. Wrapping errors in two distinct semantic ways:
//     Explain (":") adds human-readable meaning — *what* went wrong in business
//     terms; Stack (">>") adds call-path context — *where* it passed through.
//     The two are separated because interpretation and trace path answer
//     different questions and read best with different delimiters:
//     "cannot load config: file not found" vs "InitService >> LoadConfig >> ...".
//
//  2. Sentinel errors (ErrForbiddenMethod, ErrUnimplementedMethod) for the
//     common "not allowed" / "not yet implemented" conditions.
//
//  3. Precondition helpers (RequireField, RequireAny) that build a structured
//     "<component>: <field> is required" error when a constructor's inputs are
//     missing. They are the imperative counterpart to spring/conf's declarative
//     `expr:` tag: use them for cross-field rules like "addr OR service-name"
//     that a per-field tag cannot express, or anywhere a runtime input check is
//     needed outside the config-binding engine. They produce errors via Explain,
//     which is why they live in this package rather than a config-specific one.
package errutil

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrForbiddenMethod is returned when a prohibited method is called.
//
// This constant error can be used to indicate that a function or method
// must not be invoked under certain conditions (e.g., calling a private
// or restricted operation).
var ErrForbiddenMethod = errors.New("forbidden method")

// ErrUnimplementedMethod is returned when a method or operation has not yet
// been implemented.
//
// It is commonly used as a placeholder to indicate functionality that is
// intentionally left unimplemented or pending future development.
var ErrUnimplementedMethod = errors.New("unimplemented method")

// IsServerClosed reports whether err is (or wraps) http.ErrServerClosed —
// i.e. the http.Server exited because Shutdown or Close was called, which
// is a graceful stop and not a failure. Use this instead of comparing
// err == http.ErrServerClosed directly, so wrapped errors are matched too.
func IsServerClosed(err error) bool {
	return errors.Is(err, http.ErrServerClosed)
}

// Explain wraps an existing error by adding *explanatory semantics* —
// a human-readable interpretation of the underlying cause.
//
// This function represents an "explanatory wrapping" pattern. It answers
// the question: “What does this error *mean* in the current context?”
//
// Example:
//
//	err := errors.New("connection refused")
//	return errutil.Explain(err, "failed to connect to database")
//
// Output error message:
//
//	"failed to connect to database: connection refused"
//
// Core idea:
//   - Uses ":" to denote *semantic interpretation*
//   - Adds contextual meaning for upper-level business logic
//   - Transforms technical errors into understandable messages
//
// If the provided `err` is nil, Explain simply returns a new error created
// from the formatted message.
func Explain(err error, format string, args ...any) error {
	if err == nil {
		return fmt.Errorf(format, args...)
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", msg, err)
}

// Stack wraps an existing error by adding *path context* —
// an indicator of where the error has traveled in the call chain.
//
// This function represents a "stack-style" or "path-style" wrapping pattern.
// It answers the question: “Where did this error *pass through*?”
//
// Example:
//
//	err := errors.New("file not found")
//	return errutil.Stack(err, "LoadConfig")
//
// Output error message:
//
//	"LoadConfig >> file not found"
//
// Core idea:
//   - Uses ">>" to denote *path or call-trace semantics*
//   - Emphasizes the structural flow of the error (developer-oriented)
//   - Does *not* change the meaning of the underlying error
//
// Stack wrapping is useful for tracing propagation paths without
// redefining the logical meaning of the error.
//
// If the provided `err` is nil, Stack returns a new error
// constructed from the formatted message.
func Stack(err error, format string, args ...any) error {
	if err == nil {
		return fmt.Errorf(format, args...)
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s >> %w", msg, err)
}
