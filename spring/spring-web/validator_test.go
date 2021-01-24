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

package SpringWeb_test

import (
	"testing"

	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

func TestValidate(t *testing.T) {

	v := struct {
		Str string `validate:"required,len=4"`
	}{}

	v.Str = "" // 不启用参数校验器
	SpringUtils.AssertEqual(t, SpringWeb.Validate(v), nil)

	SpringWeb.SetValidator(SpringWeb.NewDefaultValidator())
	defer func() { SpringWeb.SetValidator(nil) }()

	SpringUtils.AssertPanic(t, func() {
		v.Str = ""
		if err := SpringWeb.Validate(v); err != nil {
			panic(err)
		}
	}, "'Str' Error:Field validation for 'Str' failed on the 'required' tag")

	v.Str = "1234"
	SpringUtils.AssertEqual(t, SpringWeb.Validate(v), nil)

	SpringUtils.AssertPanic(t, func() {
		v.Str = "12345"
		if err := SpringWeb.Validate(v); err != nil {
			panic(err)
		}
	}, "'Str' Error:Field validation for 'Str' failed on the 'len' tag")
}
