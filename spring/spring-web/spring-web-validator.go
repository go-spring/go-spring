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

package SpringWeb

import (
	"reflect"

	"github.com/go-playground/validator/v10"
	"github.com/go-spring/spring-utils"
)

// WebValidator 适配 gin 和 echo 的校验器接口
type WebValidator interface {
	Engine() interface{}
	Validate(i interface{}) error
	ValidateStruct(i interface{}) error
}

// Validator 全局参数校验器，没有啥好办法不做成全局变量
var Validator WebValidator = NewBuiltInValidator()

// BuiltInValidator 内置的参数校验器
type BuiltInValidator struct {
	validator *validator.Validate
}

// NewBuiltInValidator BuiltInValidator 的构造函数
func NewBuiltInValidator() *BuiltInValidator {
	return &BuiltInValidator{validator: validator.New()}
}

func (v *BuiltInValidator) Engine() interface{} {
	return v.validator
}

// Validate echo 的参数校验接口
func (v *BuiltInValidator) Validate(i interface{}) error {
	return v.validateStruct(i)
}

// ValidateStruct gin 的参数校验接口
func (v *BuiltInValidator) ValidateStruct(i interface{}) error {
	return v.validateStruct(i)
}

// validateStruct receives any kind of type, but only performed struct or pointer to struct type.
func (v *BuiltInValidator) validateStruct(i interface{}) error {
	if SpringUtils.Indirect(reflect.TypeOf(i)).Kind() == reflect.Struct {
		if err := v.validator.Struct(i); err != nil {
			return err
		}
	}
	return nil
}
