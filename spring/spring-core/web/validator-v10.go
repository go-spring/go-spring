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

package web

import (
	"reflect"

	v10 "github.com/go-playground/validator/v10"
	"github.com/go-spring/spring-core/util"
)

// defaultValidator 默认的参数校验器
type defaultValidator struct {
	validator *v10.Validate
}

// NewDefaultValidator defaultValidator 的构造函数
func NewDefaultValidator() *defaultValidator {
	return &defaultValidator{validator: v10.New()}
}

// Engine 返回原始的参数校验引擎
func (v *defaultValidator) Engine() interface{} {
	return v.validator
}

// Validate 校验参数
func (v *defaultValidator) Validate(i interface{}) error {
	if util.Indirect(reflect.TypeOf(i)).Kind() == reflect.Struct {
		return v.validator.Struct(i)
	}
	return nil
}
