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

package typeutil

import (
	"reflect"
)

// errorType is the [reflect.Type] representation of the built-in error interface.
// It is used to test whether a type is exactly `error` or implements the `error` interface.
var errorType = reflect.TypeFor[error]()

// IntType is a generic constraint that represents all signed integer types:
// int, int8, int16, int32, and int64.
type IntType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// UintType is a generic constraint that represents all unsigned integer types:
// uint, uint8, uint16, uint32, and uint64.
type UintType interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// FloatType is a generic constraint that represents floating-point types:
// float32 and float64.
type FloatType interface {
	~float32 | ~float64
}

// IsFuncType reports whether the provided reflect.Type represents a function.
func IsFuncType(t reflect.Type) bool {
	return t.Kind() == reflect.Func
}

// IsErrorType reports whether the provided reflect.Type represents the built-in
// error type or a type that implements the error interface.
func IsErrorType(t reflect.Type) bool {
	if t == nil {
		return false
	}
	return t == errorType || t.Implements(errorType)
}

// ReturnNothing reports whether the provided function type returns no values.
// It is useful when analyzing function signatures using reflection.
func ReturnNothing(t reflect.Type) bool {
	return t.NumOut() == 0
}

// ReturnOnlyError reports whether the provided function type returns exactly
// one value and that value is an error.
func ReturnOnlyError(t reflect.Type) bool {
	return t.NumOut() == 1 && IsErrorType(t.Out(0))
}

// IsConstructor reports whether the provided function type is considered
// a constructor by convention.
//
// A constructor is defined as a function that returns:
//   - one non-error value, or
//   - two values where the second value is an error.
//
// Examples of valid constructor signatures:
//
//	func() *MyStruct
//	func(cfg Config) (*MyStruct, error)
//
// Examples of invalid constructor signatures:
//
//	func()                     // returns nothing
//	func() error               // returns only an error
//	func() (*A, *B, error)     // returns more than two values
func IsConstructor(t reflect.Type) bool {
	if !IsFuncType(t) {
		return false
	}
	switch t.NumOut() {
	case 1:
		return !IsErrorType(t.Out(0))
	case 2:
		return IsErrorType(t.Out(1))
	default:
		return false
	}
}

// IsPrimitiveValueType reports whether the provided reflect.Type represents
// a primitive value type such as an integer, unsigned integer, float,
// string, or boolean.
func IsPrimitiveValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Float32, reflect.Float64:
		return true
	case reflect.String:
		return true
	case reflect.Bool:
		return true
	default:
		return false
	}
}

// IsPropBindingTarget reports whether the provided reflect.Type is a valid
// target for property binding.
//
// Valid types include:
//   - primitive value types
//   - struct types
//   - collections (map, slice, array) whose element type is a primitive value or a struct
func IsPropBindingTarget(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		// For collections, inspect the element type.
		t = t.Elem()
	default:
		// do nothing
	}
	return IsPrimitiveValueType(t) || t.Kind() == reflect.Struct
}

// IsBeanType reports whether the provided reflect.Type is considered a "bean" type.
//
// A "bean" type is defined as:
//   - a channel type
//   - a function type
//   - an interface type
//   - a pointer to a struct type
func IsBeanType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface:
		return true
	case reflect.Pointer:
		return t.Elem().Kind() == reflect.Struct
	default:
		return false
	}
}

// IsBeanInjectionTarget reports whether the provided reflect.Type is a valid
// target for bean injection.
//
// Valid targets include:
//   - a bean type itself
//   - collections (map, slice, array) whose element type is a bean
func IsBeanInjectionTarget(t reflect.Type) bool {
	if t == nil {
		return false
	}
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		// For collections, inspect the element type.
		t = t.Elem()
	default:
		// do nothing
	}
	return IsBeanType(t)
}
