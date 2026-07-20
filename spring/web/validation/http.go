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
	"context"
	"encoding/json"
	"net/http"
)

// Decoder populates a fresh *T from the request (body, query, path, ...). It is
// the only framework-specific piece of the Web seam: a gin/echo/hertz adapter
// can supply its own binder, while [JSONDecoder] covers the common JSON-body
// case with the standard library alone.
type Decoder[T any] func(*http.Request, *T) error

// JSONDecoder decodes the request body as JSON into dst. It is the default
// [Decoder] used by [Handle] when none is given.
func JSONDecoder[T any](r *http.Request, dst *T) error {
	if r.Body == nil {
		return nil
	}
	return json.NewDecoder(r.Body).Decode(dst)
}

// Handle is the server-side seam for request validation, mirroring
// aspect.NewHandler / resilience.NewHandler: it returns an [http.Handler] that
// decodes the body into a fresh T, validates it, and only then calls next with
// the populated value. A decode or validation failure short-circuits with 400
// and a structured JSON body ({"errors":[...]}) so bad input never reaches
// business code. When v is nil the value is passed through unvalidated (the seam
// stays a no-op until a validator is wired). The optional render function
// localizes messages (typically i18n-backed); when nil, each field's default
// English message is used.
func Handle[T any](v Validator, decode Decoder[T], render func(FieldError) string, next func(http.ResponseWriter, *http.Request, *T)) http.Handler {
	if decode == nil {
		decode = JSONDecoder[T]
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value T
		if err := decode(r, &value); err != nil {
			http.Error(w, "cannot decode request body", http.StatusBadRequest)
			return
		}
		if v != nil {
			if err := v.Validate(r.Context(), &value); err != nil {
				WriteError(w, err, render)
				return
			}
		}
		next(w, r, &value)
	})
}

// WriteError writes err to w as HTTP 400 with a JSON body of the form
// {"errors":["...","..."]}. When err is a [ValidationErrors] each field is
// localized through render (or its default message when render is nil); any
// other error is written as its Error() string. It is exported so adapters that
// do their own binding can reuse the exact response shape produced by [Handle].
func WriteError(w http.ResponseWriter, err error, render func(FieldError) string) {
	var msgs []string
	var errs ValidationErrors
	if as, ok := err.(ValidationErrors); ok {
		errs = as
	}
	if errs != nil {
		msgs = make([]string, len(errs))
		for i, e := range errs {
			if render != nil {
				msgs[i] = render(e)
			} else {
				msgs[i] = e.Default()
			}
		}
	} else {
		msgs = []string{err.Error()}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": msgs})
}

// Validate is a convenience over the driver registry: it resolves the driver
// registered under name, builds a [Validator] and validates v in one call. It
// is handy for the config-binding path where a struct is validated once at
// startup. Prefer building a Validator once and reusing it on hot paths.
func Validate(ctx context.Context, name string, v any) error {
	d, err := MustGetDriver(name)
	if err != nil {
		return err
	}
	val, err := d.NewValidator()
	if err != nil {
		return err
	}
	return val.Validate(ctx, v)
}
