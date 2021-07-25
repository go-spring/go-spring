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

// Package validator 提供了参数校验器接口。
package validator

// Validator 参数校验器接口。
type Validator interface {
	Validate(i interface{}) error
}

var v Validator

// Init 初始化参数校验器。
func Init(r Validator) {
	v = r
}

// InitFunc 初始化参数校验器。
func InitFunc(r func(i interface{}) error) {
	v = funcValidator(r)
}

// funcValidator 基于简单函数的参数校验器。
type funcValidator func(i interface{}) error

func (f funcValidator) Validate(i interface{}) error {
	return f(i)
}

// Validate 参数校验。
func Validate(i interface{}) error {
	if v != nil {
		return v.Validate(i)
	}
	return nil
}
