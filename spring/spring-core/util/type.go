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

// IsBeanType 返回是否是 bean 类型。在 go-spring 里，如果一个非集合类型的变量赋值
// 给另一个变量后二者指向相同的内存地址，则称这个变量的类型为 bean 类型，反之则称为
// value 类型。因此，interface、chan、func、ptr，这些都是 bean 类型，string、
// bool、int、uint、float、complex、struct，这些都是 value 类型。对于集合类型的
// 变量来说，它的类型属于 bean 类型还是 value 类型是由其元素的类型决定的，如果元素的
// 类型是 bean 类型则该变量的类型是 bean 类型，如果元素的类型是 value 类型则该变量的
// 类型是 value 类型。
func IsBeanType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr:
		return true
	case reflect.Map, reflect.Slice, reflect.Array:
		// 这里默认只发生一次递归就会停止。
		return IsBeanType(t.Elem())
	default:
		return false
	}
}

// IsValueType 返回是否是 value 类型。
func IsValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128:
		return true
	case reflect.Map,
		reflect.Slice,
		reflect.Array:
		return IsValueType(t.Elem())
	case reflect.Struct:
		// 为了减少反射的使用，这里默认用户在使用结构体时是按照值类型
		// 方式使用的，就是说结构体及其嵌套结构里面不存在任何引用类型。
		return true
	default:
		return false
	}
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

// IsConstructor 返回以函数形式注册 Bean 的函数是否合法。一个合法
// 的注册函数需要以下条件：入参可以有任意多个，支持一般形式和 Option
// 形式，返回值只能有一个或者两个，第一个返回值必须是 Bean 源，它可以是
// 结构体等值类型也可以是指针等引用类型，为值类型时内部会自动转换为引用类
// 型（获取可引用的地址），如果有第二个返回值那么它必须是 error 类型。
func IsConstructor(t reflect.Type) bool {
	returnError := t.NumOut() == 2 && IsErrorType(t.Out(1))
	return IsFuncType(t) && (t.NumOut() == 1 || returnError)
}

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
