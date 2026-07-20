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

package StarterValidation

import (
	"context"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/spring/validation"
)

type user struct {
	Email string `validate:"required,email"`
	Age   int    `validate:"min=18"`
}

// TestDriverRegisteredAsDefault proves the blank-import wiring: the "default"
// driver resolves through the neutral registry with no code change.
func TestDriverRegisteredAsDefault(t *testing.T) {
	d, err := validation.MustGetDriver("default")
	assert.Error(t, err).Nil()
	assert.That(t, d).NotNil()
}

func TestValidatePassesValidStruct(t *testing.T) {
	d, _ := validation.MustGetDriver("default")
	v, err := d.NewValidator()
	assert.Error(t, err).Nil()
	assert.Error(t, v.Validate(context.Background(), &user{Email: "a@b.com", Age: 20})).Nil()
}

// TestValidateMapsErrorsToNeutralShape checks that go-playground errors become
// validation.ValidationErrors with the tag as Rule, so "validation.<tag>" i18n
// keys resolve and the field path is preserved.
func TestValidateMapsErrorsToNeutralShape(t *testing.T) {
	d, _ := validation.MustGetDriver("default")
	v, _ := d.NewValidator()
	err := v.Validate(context.Background(), &user{Email: "not-an-email", Age: 5})
	assert.Error(t, err).NotNil()

	errs, ok := err.(validation.ValidationErrors)
	assert.That(t, ok).True()
	assert.Slice(t, []int{len(errs)}).Contains(2)

	rules := map[string]validation.FieldError{}
	for _, e := range errs {
		rules[e.Rule] = e
	}
	assert.Map(t, rules).ContainsKey("email")
	assert.Map(t, rules).ContainsKey("min")
	assert.String(t, rules["email"].Field).Equal("user.Email")
	assert.String(t, rules["min"].Param).Equal("18")
	assert.String(t, rules["email"].MessageKey()).Equal("validation.email")
}
