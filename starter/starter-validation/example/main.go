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

// Command example demonstrates go-spring.org/stdlib/validation and
// go-spring.org/stdlib/i18n end to end, using go-playground/validator via a
// blank import of starter-validation as the driver:
//
//   - config-binding path: a struct is populated with conf.Bind, then validated
//     so a misconfigured environment fails fast;
//   - Web path: an http handler decodes and validates the request body, and
//     rejects bad input with a structured, localized 400;
//   - i18n: the same ValidationErrors render in English or Chinese, loaded from
//     messages_*.yaml through the conf reader (reused, not reimplemented).
package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/i18n"
	"go-spring.org/stdlib/validation"

	_ "go-spring.org/starter-validation" // registers the "default" driver
)

// ServerConfig is bound from configuration and then validated. The validate
// tags are the same ones the Web path uses — one rule vocabulary for both.
type ServerConfig struct {
	Admin string `value:"${admin}" validate:"required,email"`
	Port  int    `value:"${port}" validate:"min=1024"`
}

// SignupRequest is the Web request body.
type SignupRequest struct {
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"min=18"`
}

func main() {
	source := loadMessages()

	fmt.Println("== config-binding path ==")
	demoConfig(source)

	fmt.Println("\n== web path (en) ==")
	demoWeb(source, "en", `{"email":"bad","age":10}`)

	fmt.Println("\n== web path (zh) ==")
	demoWeb(source, "zh", `{"email":"bad","age":10}`)

	fmt.Println("\n== web path (valid) ==")
	demoWeb(source, "en", `{"email":"a@b.com","age":20}`)
}

// loadMessages parses the bundled yaml bundles with the conf reader and feeds
// them into an in-memory i18n MapSource (default locale "en").
func loadMessages() *i18n.MapSource {
	src := i18n.NewMapSource("en")
	for locale, file := range map[string]string{"en": "messages_en.yaml", "zh": "messages_zh.yaml"} {
		m, err := reader.ReadFile(file)
		if err != nil {
			panic(err)
		}
		src.AddParsed(locale, m)
	}
	return src
}

func demoConfig(src *i18n.MapSource) {
	// A deliberately broken configuration: bad admin email, port below the floor.
	store := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
		"admin": "not-an-email",
		"port":  "80",
	}))
	var cfg ServerConfig
	if err := conf.Bind(store, &cfg, "${ROOT}"); err != nil {
		panic(err)
	}
	if err := validation.Validate(context.Background(), "default", &cfg); err != nil {
		render := i18n.Localizer(src, i18n.WithLocale(context.Background(), "en"))
		for _, line := range err.(validation.ValidationErrors).Localize(render) {
			fmt.Println(" -", line)
		}
	}
}

func demoWeb(src *i18n.MapSource, locale, body string) {
	v, _ := mustValidator()
	handler := validation.Handle[SignupRequest](v, nil,
		func(e validation.FieldError) string {
			ctx := i18n.WithLocale(context.Background(), locale)
			s, _ := src.Message(ctx, e.MessageKey(), e.Field, e.Param)
			return s
		},
		func(w http.ResponseWriter, _ *http.Request, req *SignupRequest) {
			fmt.Fprintf(w, "ok: %s", req.Email)
		},
	)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/signup", strings.NewReader(body)))
	fmt.Printf(" status=%d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
	fmt.Println()
}

func mustValidator() (validation.Validator, error) {
	d, err := validation.MustGetDriver("default")
	if err != nil {
		return nil, err
	}
	return d.NewValidator()
}
