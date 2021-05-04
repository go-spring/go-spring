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
	"context"
	"reflect"
)

const (
	valType = 1 // 值类型
	refType = 2 // 引用类型
)

var kindTypes = []uint8{
	0,       // Invalid
	valType, // Bool
	valType, // Int
	valType, // Int8
	valType, // Int16
	valType, // Int32
	valType, // Int64
	valType, // Uint
	valType, // Uint8
	valType, // Uint16
	valType, // Uint32
	valType, // Uint64
	0,       // Uintptr
	valType, // Float32
	valType, // Float64
	valType, // Complex64
	valType, // Complex128
	valType, // Array
	refType, // Chan
	refType, // Func
	refType, // Interface
	refType, // Map
	refType, // Ptr
	refType, // Slice
	valType, // String
	valType, // Struct
	0,       // UnsafePointer
}

// IsRefType 返回是否是引用类型。
func IsRefType(k reflect.Kind) bool {
	return kindTypes[k] == refType
}

// IsValueType 返回是否是值类型。
func IsValueType(k reflect.Kind) bool {
	return kindTypes[k] == valType
}

// TypeOf 获取 i 的类型。
func TypeOf(i interface{}) reflect.Type {
	switch o := i.(type) {
	case reflect.Type:
		return o
	case reflect.Value:
		return o.Type()
	default:
		return reflect.TypeOf(o)
	}
}

// TypeName 返回原始类型的全限定名，Go 语言允许不同的路径下存在相同的包，因此有全限定名
// 的需求，形如 "github.com/go-spring/spring-core/SpringCore.BeanDefinition"。
func TypeName(i interface{}) string {
	typ := TypeOf(i)

	for { // 去掉指针和数组的包装，以获得原始类型
		if k := typ.Kind(); k == reflect.Ptr || k == reflect.Slice {
			typ = typ.Elem()
		} else {
			break
		}
	}

	if pkgPath := typ.PkgPath(); pkgPath != "" {
		return pkgPath + "/" + typ.String()
	}
	return typ.String() // 内置类型的路径为空
}

var (

	// errorType error 的反射类型。
	errorType = reflect.TypeOf((*error)(nil)).Elem()

	// contextType context.Context 的反射类型。
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()
)

// IsFuncType t 是否是 func 类型。
func IsFuncType(t reflect.Type) bool {
	return t.Kind() == reflect.Func
}

// IsErrorType t 是否是 error 类型。
func IsErrorType(t reflect.Type) bool {
	return t == errorType
}

// IsContextType t 是否是 context.Context 类型。
func IsContextType(t reflect.Type) bool {
	return t == contextType
}

// ReturnNothing 函数是否无返回值。
func ReturnNothing(t reflect.Type) bool {
	return t.NumOut() == 0
}

// ReturnOnlyError 函数是否只返回错误值。
func ReturnOnlyError(t reflect.Type) bool {
	return t.NumOut() == 1 && IsErrorType(t.Out(0))
}

// HasReceiver 函数是否具有接收者。
func HasReceiver(t reflect.Type, receiver reflect.Type) bool {
	return t.NumIn() >= 1 && t.In(0) == receiver
}
