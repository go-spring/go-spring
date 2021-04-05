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

package util

import (
	"reflect"
)

// errorType error 的反射类型。
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// FuncType t 是否是 func 类型。
func FuncType(t reflect.Type) bool {
	return t.Kind() == reflect.Func
}

// ErrorType t 是否是 error 类型。
func ErrorType(t reflect.Type) bool {
	return t == errorType
}

// ReturnNothing 函数是否无返回值。
func ReturnNothing(t reflect.Type) bool {
	return t.NumOut() == 0
}

// ReturnOnlyError 函数是否只返回错误值。
func ReturnOnlyError(t reflect.Type) bool {
	return t.NumOut() == 1 && ErrorType(t.Out(0))
}

// WithReceiver 函数是否具有接收者。
func WithReceiver(t reflect.Type, receiver reflect.Type) bool {
	return t.NumIn() >= 1 && t.In(0) == receiver
}
