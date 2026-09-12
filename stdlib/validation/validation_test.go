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

package validation

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestFieldErrorMessageKeyAndDefault(t *testing.T) {
	e := FieldError{Field: "User.Email", Rule: "email"}
	assert.String(t, e.MessageKey()).Equal("validation.email")
	assert.String(t, e.Default()).Contains(`"User.Email"`)

	withParam := FieldError{Field: "User.Age", Rule: "min", Param: "18"}
	assert.String(t, withParam.Default()).Contains("18")
}

func TestValidationErrorsError(t *testing.T) {
	assert.String(t, ValidationErrors{}.Error()).Equal("validation: no errors")
	es := ValidationErrors{{Field: "A", Rule: "required"}, {Field: "B", Rule: "email"}}
	assert.String(t, es.Error()).Contains("required")
	assert.String(t, es.Error()).Contains("email")
}

func TestLocalizeUsesMsgThenFallsBackToDefault(t *testing.T) {
	es := ValidationErrors{
		{Field: "Email", Rule: "email"},
		{Field: "Age", Rule: "min", Param: "18"},
	}
	// msg knows only the email key; the min key falls back to Default.
	msg := func(key string, args ...any) string {
		if key == "validation.email" {
			return "invalid email"
		}
		return ""
	}
	got := es.Localize(msg)
	assert.Slice(t, got).Length(2)
	assert.String(t, got[0]).Equal("invalid email")
	assert.String(t, got[1]).Contains("min") // fell back to Default

	// A nil msg localizes everything via Default.
	nilGot := es.Localize(nil)
	assert.String(t, nilGot[0]).Contains("email")
}

func TestLocalizeCustomCallback(t *testing.T) {
	es := ValidationErrors{
		{Field: "S.Password", Rule: "min", Param: "8", Kind: "string"},
		{Field: "S.Age", Rule: "min", Param: "18", Kind: "int"},
		{Field: "S.Email", Rule: "email"},
	}
	msg := func(key string, args ...any) string {
		if key == "validation.min" {
			return args[0].(string) + " 至少为 " + args[1].(string)
		}
		return "" // email has no translation
	}
	custom := func(e FieldError) string {
		if e.Rule == "min" && e.Kind == "string" {
			return e.Field + " 长度至少为 " + e.Param
		}
		return ""
	}
	got := es.Localize(msg, WithCustom(custom))
	assert.String(t, got[0]).Equal("S.Password 长度至少为 8") // custom: kind branch
	assert.String(t, got[1]).Equal("S.Age 至少为 18")       // msg template
	assert.String(t, got[2]).Contains("email")           // Default fallback

	// Without custom, everything runs the generic path.
	plain := es.Localize(msg)
	assert.String(t, plain[0]).Equal("S.Password 至少为 8")
}
