/*
 * Copyright 2024 The Go-Spring Authors.
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

package conf

import (
	"maps"

	"github.com/expr-lang/expr"
	"go-spring.org/stdlib/errutil"
)

var validateFuncs = map[string]any{}

// ValidateFunc defines a type for validation functions.
type ValidateFunc[T any] func(T) (bool, error)

// RegisterValidateFunc registers a validation function with a specific name.
// Must be called in init functions only.
func RegisterValidateFunc[T any](name string, fn ValidateFunc[T]) {
	if name == "" {
		panic("validate function name can't be empty")
	}
	if fn == nil {
		panic("validate function " + name + " cannot be nil")
	}
	if _, ok := validateFuncs[name]; ok {
		panic("validate function " + name + " already exists")
	}
	validateFuncs[name] = fn
}

// validateField validates a field using a validation expression (tag) and the field value (i).
// It evaluates the expression and checks if the result is true (i.e., the validation passes).
// If any error occurs during evaluation or if the validation fails, an error is returned.
func validateField(tag string, i any) error {
	env := map[string]any{"$": i}
	maps.Copy(env, validateFuncs)
	r, err := expr.Eval(tag, env)
	if err != nil {
		return err
	}
	ret, ok := r.(bool)
	if !ok {
		return errutil.Explain(nil, "expression must return a boolean value")
	}
	if !ret {
		return errutil.Explain(nil, "expression evaluated to false")
	}
	return nil
}
