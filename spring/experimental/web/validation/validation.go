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

// Package validation defines a framework-agnostic, zero-dependency abstraction
// for struct validation, split from its implementations exactly like
// [go-spring.org/spring/resilience] and [go-spring.org/spring/discovery].
//
// It answers one question — "is this struct well-formed?" — for the two binding
// paths in the framework:
//
//   - configuration binding (conf.Bind): validate settings once at startup so a
//     misconfigured environment fails fast rather than misbehaving later;
//   - inbound Web requests: validate the decoded request body and reject bad
//     input with a structured 400 before it reaches business code.
//
// The [Validator] seam is backend-neutral. A [Driver] turns nothing more than a
// name into a live Validator; the recommended production driver wraps
// go-playground/validator and lives in its own module (starter-validation),
// registering itself on blank import. stdlib carries no third-party validator so
// the foundation layer keeps its zero-dependency guarantee.
//
// Failures surface as [ValidationErrors]: a flat list of [FieldError] values
// that name the field, the rule that failed and its parameter. Because the same
// list must render in whatever language the caller speaks, validation does not
// import an i18n package; instead [ValidationErrors.Localize] takes a message
// lookup function, letting [go-spring.org/spring/i18n] (or anything else) plug
// in without a hard dependency.
package validation

import (
	"context"
	"fmt"
	"strings"
)

// FieldError describes a single rule that failed on one field. It is the neutral
// unit every driver must produce, independent of the underlying validator.
type FieldError struct {
	// Field is the location of the offending value, using the struct field path
	// (e.g. "User.Email"). It is the most stable identifier for the failure.
	Field string

	// Rule is the name of the rule that failed (e.g. "required", "email",
	// "min"). It doubles as the stem of the i18n message key.
	Rule string

	// Param is the rule's parameter when it has one (e.g. "3" for min=3) and is
	// empty otherwise. It is exposed to messages as an argument.
	Param string

	// Value is the actual value that failed, best-effort, for logging and
	// message interpolation. Drivers may leave it nil.
	Value any
}

// MessageKey returns the i18n key convention for the failed rule: the rule name
// prefixed with "validation.". A driver that reports Rule "email" yields the key
// "validation.email", which an i18n message source resolves to a localized
// template.
func (e FieldError) MessageKey() string {
	return "validation." + e.Rule
}

// Default renders a plain, English, dependency-free message for the failure. It
// is the fallback used when no i18n message source is wired, so a bare
// validation error is still readable.
func (e FieldError) Default() string {
	if e.Param != "" {
		return fmt.Sprintf("field %q failed rule %q (%s)", e.Field, e.Rule, e.Param)
	}
	return fmt.Sprintf("field %q failed rule %q", e.Field, e.Rule)
}

// ValidationErrors aggregates every field failure from one [Validator.Validate]
// call. A nil or empty ValidationErrors means the value is valid; drivers must
// return a nil error (not an empty ValidationErrors) on success so callers can
// test with a plain err != nil.
type ValidationErrors []FieldError

// Error implements the error interface by joining each field's default message.
func (es ValidationErrors) Error() string {
	if len(es) == 0 {
		return "validation: no errors"
	}
	parts := make([]string, len(es))
	for i, e := range es {
		parts[i] = e.Default()
	}
	return "validation: " + strings.Join(parts, "; ")
}

// Localize renders every field error to a human string using msg, a message
// lookup bound to a locale (typically i18n.MessageSource.Message curried with a
// ctx). msg receives the field's [FieldError.MessageKey] and the arguments
// (field name, then param) so a template like "{0} must be at least {1}" can be
// filled. It is kept a plain function so this package never imports an i18n
// implementation. When msg returns an empty string for a key (missing
// translation), the field's [FieldError.Default] is used so output is never
// blank.
func (es ValidationErrors) Localize(msg func(key string, args ...any) string) []string {
	out := make([]string, len(es))
	for i, e := range es {
		var s string
		if msg != nil {
			s = msg(e.MessageKey(), e.Field, e.Param)
		}
		if s == "" {
			s = e.Default()
		}
		out[i] = s
	}
	return out
}

// Validator checks that a value satisfies its declared rules. Implementations
// must be safe for concurrent use and must return a nil error when the value is
// valid, or a [ValidationErrors] (via the error interface) describing every
// failure otherwise.
type Validator interface {
	Validate(ctx context.Context, v any) error
}

// Driver builds a [Validator]. Backends implement it and register under a name
// via [RegisterDriver]; callers select one by name through [MustGetDriver] with
// no per-backend adaptation.
type Driver interface {
	NewValidator() (Validator, error)
}
