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
	"net/http/httptest"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// stubValidator returns a fixed ValidationErrors (or nil) regardless of input,
// letting the seam tests run without a real driver.
type stubValidator struct {
	errs ValidationErrors
}

func (s stubValidator) Validate(context.Context, any) error {
	if len(s.errs) == 0 {
		return nil
	}
	return s.errs
}

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

func TestRegistryPanicsAndLookup(t *testing.T) {
	assert.Panic(t, func() { RegisterDriver("", nil) }, "empty name")
	assert.Panic(t, func() { RegisterDriver("x", nil) }, "nil driver")

	RegisterDriver("test-drv", driverFunc(func() (Validator, error) {
		return stubValidator{}, nil
	}))
	assert.Panic(t, func() {
		RegisterDriver("test-drv", driverFunc(func() (Validator, error) { return nil, nil }))
	}, "already registered")

	d, err := MustGetDriver("test-drv")
	assert.Error(t, err).Nil()
	assert.That(t, d).NotNil()

	_, err = MustGetDriver("missing")
	assert.Error(t, err).NotNil()
	assert.String(t, err.Error()).Contains("no driver registered")
}

type driverFunc func() (Validator, error)

func (f driverFunc) NewValidator() (Validator, error) { return f() }

func TestValidateConvenience(t *testing.T) {
	RegisterDriver("conv", driverFunc(func() (Validator, error) {
		return stubValidator{errs: ValidationErrors{{Field: "X", Rule: "required"}}}, nil
	}))
	err := Validate(context.Background(), "conv", struct{}{})
	assert.Error(t, err).NotNil()

	err = Validate(context.Background(), "nope", struct{}{})
	assert.Error(t, err).NotNil()
}

type payload struct {
	Name string `json:"name"`
}

func TestHandlePassesValidValue(t *testing.T) {
	h := Handle[payload](stubValidator{}, nil, nil, func(w http.ResponseWriter, _ *http.Request, p *payload) {
		_, _ = w.Write([]byte("ok:" + p.Name))
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"alice"}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusOK)
	assert.String(t, rec.Body.String()).Equal("ok:alice")
}

func TestHandleRejectsInvalidValue(t *testing.T) {
	sv := stubValidator{errs: ValidationErrors{{Field: "Name", Rule: "required"}}}
	h := Handle[payload](sv, nil, nil, func(http.ResponseWriter, *http.Request, *payload) {
		t.Fatal("next must not be called on validation failure")
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusBadRequest)

	var body struct {
		Errors []string `json:"errors"`
	}
	assert.Error(t, json.Unmarshal(rec.Body.Bytes(), &body)).Nil()
	assert.Slice(t, body.Errors).Length(1)
}

func TestHandleRejectsBadJSON(t *testing.T) {
	h := Handle[payload](stubValidator{}, nil, nil, func(http.ResponseWriter, *http.Request, *payload) {
		t.Fatal("next must not be called on decode failure")
	})
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{bad`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Number(t, rec.Code).Equal(http.StatusBadRequest)
}

func TestWriteErrorRendersWithCustomRenderer(t *testing.T) {
	rec := httptest.NewRecorder()
	errs := ValidationErrors{{Field: "Email", Rule: "email"}}
	WriteError(rec, errs, func(e FieldError) string { return "custom:" + e.Rule })
	assert.String(t, rec.Body.String()).Contains("custom:email")

	// A non-ValidationErrors error is written as its Error() text.
	rec2 := httptest.NewRecorder()
	WriteError(rec2, context.Canceled, nil)
	assert.String(t, rec2.Body.String()).Contains("context canceled")
}
