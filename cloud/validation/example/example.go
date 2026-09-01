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

// Command example demonstrates the validation error model: run
// go-playground/validator directly, map its failures onto
// validation.ValidationErrors, then render the same failure list in two
// languages through cloud/i18n.
//
// It self-asserts both renderings and exits non-zero on mismatch, so it doubles
// as the package's smoke test. No external services are required.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"

	"go-spring.org/cloud/i18n"
	"go-spring.org/cloud/validation"
)

// SignUp is the request DTO under validation. The validate tags belong to
// go-playground/validator; validation knows nothing about them.
type SignUp struct {
	Email    string `json:"email"    validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Age      int    `json:"age"      validate:"min=18"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("validation example ok")
}

func run() error {
	// 1. Validate with go-playground/validator, as any app would.
	v := validator.New(validator.WithRequiredStructEnabled())
	err := v.Struct(&SignUp{Email: "not-an-email", Password: "short", Age: 5})

	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return fmt.Errorf("unexpected error: %w", err)
	}

	// 2. Map the failures onto the neutral model — a few caller-side lines.
	// Kind disambiguates rules like min: a length on strings, a value on
	// numbers.
	out := make(validation.ValidationErrors, len(verrs))
	for i, fe := range verrs {
		out[i] = validation.FieldError{
			Field: fe.Namespace(),
			Rule:  fe.Tag(),
			Param: fe.Param(),
			Kind:  fe.Kind().String(),
		}
	}
	if len(out) != 3 {
		return fmt.Errorf("want 3 field errors, got %d", len(out))
	}

	// 3. Localize the same list per request locale. Message keys follow the
	// "validation.<rule>" convention; missing translations fall back to
	// FieldError.Default().
	src := i18n.NewMapSource(i18n.WithFallbackLocale("en")).
		Add("en", "validation.email", "{0} is not a valid email address").
		Add("en", "validation.min", "{0} must be at least {1}").
		Add("en", "validation.required", "{0} is required").
		Add("zh-CN", "validation.email", "{0} 不是合法邮箱").
		Add("zh-CN", "validation.min", "{0} 至少为 {1}").
		Add("zh-CN", "validation.required", "{0} 必填")

	for _, tc := range []struct {
		locale string
		want   string
	}{
		{"zh-CN", "SignUp.Password 至少为 8"},
		{"en", "SignUp.Password must be at least 8"},
	} {
		ctx := i18n.WithLocale(context.Background(), tc.locale)
		msgs := out.Localize(i18n.Localizer(src, ctx))
		if msgs[1] != tc.want {
			return fmt.Errorf("locale %s: want %q, got %q", tc.locale, tc.want, msgs[1])
		}
		fmt.Printf("[%s] %v\n", tc.locale, msgs)
	}

	// 3b. The optional custom callback overrides per field — here one field
	// gets dedicated phrasing and min-on-string is disambiguated from
	// min-on-number via Kind. Returning "" falls through to the generic
	// template.
	custom := func(fe validation.FieldError) string {
		if fe.Field == "SignUp.Password" {
			return "密码至少 8 位"
		}
		if fe.Rule == "min" && fe.Kind == "string" {
			return fmt.Sprintf("%s 长度至少为 %s", fe.Field, fe.Param)
		}
		return ""
	}
	base := i18n.Localizer(src, i18n.WithLocale(context.Background(), "zh-CN"))
	msgs := out.Localize(base, validation.WithCustom(custom))
	if msgs[1] != "密码至少 8 位" || msgs[2] != "SignUp.Age 至少为 18" {
		return fmt.Errorf("custom rendering: unexpected %v", msgs)
	}
	fmt.Println("[custom]", msgs)

	// 4. The list is also a plain error for logs and non-localized paths.
	fmt.Println("plain:", out)
	return nil
}
