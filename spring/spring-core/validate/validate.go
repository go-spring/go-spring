/*
 * Copyright 2012-2019 the original author or authors.
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

package validate

import (
	"fmt"

	"github.com/go-spring/spring-core/expr"
)

var Validator Interface = &exprValidator{}

// Interface is the minimal interface for validating a variable or struct.
type Interface interface {
	TagName() string
	Struct(i interface{}) error
	Field(i interface{}, tag string) error
}

// TagName returns the validator's tag.
func TagName() string {
	return Validator.TagName()
}

// Struct validates the exposed fields of struct, and automatically validates
// the nested structs, unless otherwise specified.
func Struct(i interface{}) error {
	return Validator.Struct(i)
}

// Field validates a single variable.
func Field(i interface{}, tag string) error {
	if tag == "" {
		return nil
	}
	return Validator.Field(i, tag)
}

type exprValidator struct{}

// TagName returns the validator's tag.
func (d exprValidator) TagName() string {
	return "expr"
}

// Struct validates the exposed fields and the nested structs of struct.
func (d exprValidator) Struct(i interface{}) error {
	return nil
}

// Field validates a single variable.
func (d exprValidator) Field(i interface{}, tag string) error {
	ok, err := expr.Eval(tag, i)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("validate failed on %q for value %v", tag, i)
	}
	return nil
}
