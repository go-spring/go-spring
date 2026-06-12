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

// Package errutil provides lightweight utilities for structuring and wrapping errors
// in two distinct semantic ways:
//
//  1. Explanatory wrapping — adds human-readable meaning or interpretation to an error.
//     It clarifies *what* went wrong in business or logical terms.
//
//  2. Stack (or Path) wrapping — adds contextual call-path information, showing *where*
//     in the call chain the error was passed through.
//
// These two patterns serve different purposes:
//
//   - Explanatory errors are user-facing or semantic: "cannot load configuration: file not found".
//   - Stack errors are developer-facing or structural: "InitService >> LoadConfig >> file not found".
//
// The goal is to make error wrapping more expressive by separating *interpretation* (":")
// from *trace path* (">>").
package errutil

import (
	"errors"
	"fmt"
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
