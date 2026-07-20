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

// Package StarterValidation registers go-playground/validator as the default
// validation driver for the abstraction defined in
// [go-spring.org/spring/validation].
//
// It is enabled purely by a blank import:
//
//	import _ "go-spring.org/starter-validation"
//
// After that, validation.MustGetDriver("default") returns a [validation.Driver]
// backed by go-playground/validator, so both binding paths (conf.Bind and Web
// request handling) can validate structs tagged with `validate:"..."`. The
// third-party validator is pulled in here — never in stdlib — so the foundation
// layer keeps its zero-dependency guarantee, exactly like starter-resilience
// does for sentinel-golang.
//
// Errors are translated into the neutral [validation.ValidationErrors]: the
// validator tag becomes [validation.FieldError.Rule] (so the i18n key
// "validation.<tag>" resolves), the struct field path becomes Field, and the
// tag parameter becomes Param.
package StarterValidation

import (
	"context"

	"github.com/go-playground/validator/v10"

	"go-spring.org/spring/web/validation"
)

func init() {
	validation.RegisterDriver("default", playgroundDriver{})
}

type playgroundDriver struct{}

func (playgroundDriver) NewValidator() (validation.Validator, error) {
	return &playgroundValidator{v: validator.New(validator.WithRequiredStructEnabled())}, nil
}

type playgroundValidator struct {
	v *validator.Validate
}

// Validate runs go-playground/validator and maps its result onto the neutral
// [validation.ValidationErrors]. A nil error means valid. An InvalidValidationError
// (e.g. a nil pointer) is returned verbatim since it is a programming error, not
// a field failure.
func (p *playgroundValidator) Validate(_ context.Context, v any) error {
	err := p.v.Struct(v)
	if err == nil {
		return nil
	}
	var invalid *validator.InvalidValidationError
	if as, ok := err.(*validator.InvalidValidationError); ok {
		invalid = as
		return invalid
	}
	verrs, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}
	out := make(validation.ValidationErrors, len(verrs))
	for i, fe := range verrs {
		out[i] = validation.FieldError{
			Field: fe.Namespace(),
			Rule:  fe.Tag(),
			Param: fe.Param(),
			Value: fe.Value(),
		}
	}
	return out
}
