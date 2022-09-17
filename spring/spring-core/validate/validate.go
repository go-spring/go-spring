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

type Interface interface {
	TagName() string
	Struct(i interface{}) error
	Field(i interface{}, tag string) error
}

var Validator Interface = &Validate{}

func TagName() string {
	return Validator.TagName()
}

func Struct(i interface{}) error {
	return Validator.Struct(i)
}

func Field(i interface{}, tag string) error {
	return Validator.Field(i, tag)
}

type Validate struct{}

func (d Validate) TagName() string {
	return "expr"
}

func (d Validate) Struct(i interface{}) error {
	return nil
}

func (d Validate) Field(i interface{}, tag string) error {
	if tag == "" {
		return nil
	}
	ok, err := expr.Eval(tag, i)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("validate failed on %q for value %v", tag, i)
	}
	return nil
}
