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

//go:generate mockgen -build_flags="-mod=mod" -package=util -source=type.go -destination=type_mock.go

package util

import (
	"context"
	"reflect"
	"strings"
)

// errorType the reflection type of error.
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// contextType the reflection type of context.Context.
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// TypeName returns a fully qualified name consisting of package path and type name.
func TypeName(i interface{}) string {

	var typ reflect.Type
	switch o := i.(type) {
	case reflect.Type:
		typ = o
	case reflect.Value:
		typ = o.Type()
	default:
		typ = reflect.TypeOf(o)
	}

	for {
		if k := typ.Kind(); k == reflect.Ptr || k == reflect.Slice {
			typ = typ.Elem()
		} else {
			break
		}
	}

	if pkgPath := typ.PkgPath(); pkgPath != "" {
		pkgPath = strings.TrimSuffix(pkgPath, "_test")
		return pkgPath + "/" + typ.String()
	}
	return typ.String() // the path of built-in type is empty
}

// A BeanSelector can be the ID of a bean, a `reflect.Type`, a pointer such as
// `(*error)(nil)`, or a BeanDefinition value.
type BeanSelector interface{}

// A BeanDefinition describes a bean whose lifecycle is managed by IoC container.
type BeanDefinition interface {
	Type() reflect.Type
	Value() reflect.Value
	Interface() interface{}
	ID() string
	BeanName() string
	TypeName() string
	Created() bool
	Wired() bool
}

// Converter converts string value into user-defined value. It should be function
// type, and its prototype is func(string)(type,error).
type Converter interface{}

// IsConverter returns whether `t` is a converter type.
func IsConverter(t reflect.Type) bool {
	return IsFuncType(t) &&
		t.NumIn() == 1 &&
		t.In(0).Kind() == reflect.String &&
		t.NumOut() == 2 &&
		(IsValueType(t.Out(0)) || IsFuncType(t.Out(0))) && IsErrorType(t.Out(1))
}

// IsFuncType returns whether `t` is func type.
func IsFuncType(t reflect.Type) bool {
	return t.Kind() == reflect.Func
}

// IsErrorType returns whether `t` is error type.
func IsErrorType(t reflect.Type) bool {
	return t == errorType || t.Implements(errorType)
}

// IsContextType returns whether `t` is context.Context type.
func IsContextType(t reflect.Type) bool {
	return t == contextType || t.Implements(contextType)
}

// ReturnNothing returns whether the function has no return value.
func ReturnNothing(t reflect.Type) bool {
	return t.NumOut() == 0
}

// ReturnOnlyError returns whether the function returns only error value.
func ReturnOnlyError(t reflect.Type) bool {
	return t.NumOut() == 1 && IsErrorType(t.Out(0))
}

// IsStructPtr returns whether it is the pointer type of structure.
func IsStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

// IsConstructor returns whether `t` is a constructor type. What is a constructor?
// It should be a function first, has any number of inputs and supports the option
// pattern input, has one or two outputs and the second output should be an error.
func IsConstructor(t reflect.Type) bool {
	returnError := t.NumOut() == 2 && IsErrorType(t.Out(1))
	return IsFuncType(t) && (t.NumOut() == 1 || returnError)
}

// HasReceiver returns whether the function has a receiver.
func HasReceiver(t reflect.Type, receiver reflect.Value) bool {
	if t.NumIn() < 1 {
		return false
	}
	t0 := t.In(0)
	if t0.Kind() != reflect.Interface {
		return t0 == receiver.Type()
	}
	return receiver.Type().Implements(t0)
}

// IsPrimitiveValueType returns whether `t` is the primitive value type which only is
// int, unit, float, bool, string and complex.
func IsPrimitiveValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Complex64, reflect.Complex128:
		return true
	case reflect.Float32, reflect.Float64:
		return true
	case reflect.String:
		return true
	case reflect.Bool:
		return true
	}
	return false
}

// IsValueType returns whether the input type is the primitive value type and their
// composite type including array, slice, map and struct, such as []int, [3]string,
// []string, map[int]int, map[string]string, etc.
func IsValueType(t reflect.Type) bool {
	fn := func(t reflect.Type) bool {
		return IsPrimitiveValueType(t) || t.Kind() == reflect.Struct
	}
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return fn(t.Elem())
	default:
		return fn(t)
	}
}

// IsBeanType returns whether `t` is a bean type.
func IsBeanType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface:
		return true
	case reflect.Ptr:
		return t.Elem().Kind() == reflect.Struct
	default:
		return false
	}
}

// IsBeanReceiver returns whether the `t` is a bean receiver, a bean receiver can
// be a bean, a map or slice whose elements are beans.
func IsBeanReceiver(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return IsBeanType(t.Elem())
	default:
		return IsBeanType(t)
	}
}
