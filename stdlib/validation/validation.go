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

// Package validation is the neutral error model for struct validation: a flat
// [ValidationErrors] list of [FieldError] values naming the field, the rule
// that failed and its parameter. Run whichever validator you like (typically
// go-playground/validator); when it fails, map its errors onto this shape so
// they render uniformly and localize.
//
// [FieldError.MessageKey] derives a message key from the rule name
// ("validation." + rule), and [ValidationErrors.Localize] takes a plain lookup
// function, so any message source — [go-spring.org/stdlib/i18n] or your own —
// plugs in without this package depending on it.
package validation

import (
	"fmt"
	"strings"
)

// FieldError describes a single rule that failed on one field.
type FieldError struct {
	// Field is the location of the offending value, using the struct field path
	// (e.g. "User.Email"). It is the most stable identifier for the failure.
	Field string

	// Rule is the name of the rule that failed (e.g. "required", "email",
	// "min"). It doubles as the stem of the i18n message key.
	Rule string

	// Param is the rule's parameter when it has one (e.g. "3" for min=3) and is
	// empty otherwise. Messages receive it as the second argument, after the
	// field name.
	Param string

	// Kind is the field's kind ("string", "int", ...) when the mapper knows it,
	// empty otherwise. The package never reads it; it is there so a [WithCustom]
	// callback can tell apart rules whose meaning depends on the type (min is a
	// length on strings but a value on numbers).
	Kind string

	// Value is the value that failed, when the mapper supplies it, and may be
	// left empty. The package never reads it — [ValidationErrors.Localize] does
	// not interpolate it into templates; it reaches the caller's own logging and
	// the [WithCustom] callback.
	Value any
}

// MessageKey returns the message key for the failed rule: the rule name
// prefixed with "validation.", so rule "email" yields "validation.email". An
// empty Rule yields the bare prefix "validation.".
func (e FieldError) MessageKey() string {
	return "validation." + e.Rule
}

// Default renders a plain English message for the failure. It is the last step
// of [ValidationErrors.Localize] and the text [ValidationErrors.Error] joins, so
// a failure stays readable with no message source wired.
func (e FieldError) Default() string {
	if e.Param != "" {
		return fmt.Sprintf("field %q failed on the %q rule (param %s)", e.Field, e.Rule, e.Param)
	}
	return fmt.Sprintf("field %q failed on the %q rule", e.Field, e.Rule)
}

// ValidationErrors aggregates every field failure from one validation run. A
// valid value yields no list at all: report success with a nil error rather
// than an empty list, so callers can test with a plain err != nil.
type ValidationErrors []FieldError

// Error implements the error interface, joining each field's
// [FieldError.Default] with "; " under a "validation: " prefix. The empty list
// renders as "validation: no errors".
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

// LocalizeOption customizes [ValidationErrors.Localize].
type LocalizeOption func(*localizeOptions)

type localizeOptions struct {
	custom func(FieldError) string
}

// WithCustom overrides per-field rendering: custom receives the whole
// [FieldError] and wins over the message template when it returns a non-empty
// string. Use it to give one field dedicated phrasing, or to branch on
// [FieldError.Kind] for rules that mean different things on different types
// (min is a length on strings but a value on numbers). Returning "" hands the
// field to the template path.
func WithCustom(custom func(FieldError) string) LocalizeOption {
	return func(o *localizeOptions) { o.custom = custom }
}

// Localize renders every field error to a human string. Each error resolves
// through up to three steps, stopping at the first non-empty result:
//
//  1. the [WithCustom] callback, when one was given;
//  2. msg, a message lookup bound to a locale — typically the function
//     [go-spring.org/stdlib/i18n.Localizer] returns. It receives the field's
//     [FieldError.MessageKey] and the arguments (field name, then param), so a
//     template like "{0} must be at least {1}" can be filled. A nil msg skips
//     this step;
//  3. [FieldError.Default], so the result is never blank.
//
// The returned slice is parallel to es: one string per [FieldError].
func (es ValidationErrors) Localize(msg func(key string, args ...any) string, opts ...LocalizeOption) []string {
	var o localizeOptions
	for _, opt := range opts {
		opt(&o)
	}
	out := make([]string, len(es))
	for i, e := range es {
		var s string
		if o.custom != nil {
			s = o.custom(e)
		}
		if s == "" && msg != nil {
			s = msg(e.MessageKey(), e.Field, e.Param)
		}
		if s == "" {
			s = e.Default()
		}
		out[i] = s
	}
	return out
}
