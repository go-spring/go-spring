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

// Command example demonstrates cloud/i18n end to end: locale carried on the
// context, per-locale resolution with default-locale fallback, a nested bundle
// ingested via AddParsed, missing-key behaviour, and the Localizer pairing
// with validation.ValidationErrors.Localize.
//
// It self-asserts every expectation and exits non-zero on mismatch, so it
// doubles as the package's smoke test. No external services are required.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"go-spring.org/cloud/i18n"
	"go-spring.org/cloud/validation"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "example failed:", err)
		os.Exit(1)
	}
	fmt.Println("i18n example ok")
}

func run() error {
	// 1. The bundle: en as the fallback locale, explicit templates for en and
	// zh, plus a nested map in the shape a yaml/json reader produces.
	src := i18n.NewMapSource(i18n.WithFallbackLocale("en")).
		Add("en", "greet", "Hello, {0}!").
		Add("zh", "greet", "你好, {0}!").
		Add("zh", "farewell", "再见, {0}!")

	src.AddParsed("zh", map[string]any{
		"validation": map[string]any{
			"email": "{0} 不是合法邮箱",
		},
	})

	// 2. Resolve per request locale (middleware would set this once from
	// Accept-Language).
	zh := i18n.WithLocale(context.Background(), "zh")
	greet, err := src.Message(zh, "greet", "Go-Spring")
	if err != nil || greet != "你好, Go-Spring!" {
		return fmt.Errorf("greet: %q, %v", greet, err)
	}

	// 3. Missing key: farewell exists only in zh, so an en request finds it in
	// neither the request locale nor the fallback locale — the key itself comes
	// back with an error wrapping ErrMessageNotFound.
	en := i18n.WithLocale(context.Background(), "en")
	msg, err := src.Message(en, "farewell", "Go-Spring")
	if !errors.Is(err, i18n.ErrMessageNotFound) || msg != "farewell" {
		return fmt.Errorf("missing key: %q, %v", msg, err)
	}
	fmt.Println("missing ->", msg, err)

	// 4. The nested map landed under a dot-joined key.
	msg, err = src.Message(zh, "validation.email", "SignUp.Email")
	if err != nil || msg != "SignUp.Email 不是合法邮箱" {
		return fmt.Errorf("AddParsed key: %q, %v", msg, err)
	}

	// 5. The validation pairing: Localizer curries the source into the
	// lookup shape Localize consumes; a missing key yields "" so Localize
	// falls back to FieldError.Default().
	errs := validation.ValidationErrors{
		{Field: "SignUp.Email", Rule: "email"},
		{Field: "SignUp.Age", Rule: "min", Param: "18"},
	}
	msgs := errs.Localize(i18n.Localizer(src, zh))
	if msgs[0] != "SignUp.Email 不是合法邮箱" {
		return fmt.Errorf("localized: %q", msgs[0])
	}
	if msgs[1] != `field "SignUp.Age" failed rule "min" (18)` {
		return fmt.Errorf("default fallback: %q", msgs[1])
	}

	fmt.Printf("greet(zh) -> %s\n", greet)
	fmt.Printf("localized -> %v\n", msgs)
	return nil
}
