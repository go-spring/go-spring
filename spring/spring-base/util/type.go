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

// errorType error 的反射类型。
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// contextType context.Context 的反射类型。
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// BeanSelector bean 选择器，可以是 bean ID 字符串，可
// 以是 reflect.Type 对象，可以是形如 (*error)(nil)
// 的指针，还可以是 Definition 类型的对象。
type BeanSelector interface{}

// BeanDefinition bean 元数据。
type BeanDefinition interface {
	Type() reflect.Type     // 类型
	Value() reflect.Value   // 值
	Interface() interface{} // 源
	ID() string             // 返回 bean 的 ID
	BeanName() string       // 返回 bean 的名称
	TypeName() string       // 返回类型的全限定名
	Created() bool          // 返回是否已创建
	Wired() bool            // 返回是否已注入
}

// Converter converts string value into user-defined value. It should
// be function type, and its prototype is func(string)(type,error).
type Converter interface{}

// IsValidConverter 返回是否是合法的转换器类型。
func IsValidConverter(t reflect.Type) bool {
	return t.Kind() == reflect.Func &&
		t.NumIn() == 1 &&
		t.In(0).Kind() == reflect.String &&
		t.NumOut() == 2 &&
		(IsValueType(t.Out(0)) || IsFuncType(t.Out(0))) && IsErrorType(t.Out(1))
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

// IsConstructor 返回以函数形式注册 bean 的函数是否合法。一个合法
// 的注册函数需要以下条件：入参可以有任意多个，支持一般形式和 Option
// 形式，返回值只能有一个或者两个，第一个返回值必须是 bean 源，它可以是
// 结构体等值类型也可以是指针等引用类型，为值类型时内部会自动转换为引用类
// 型（获取可引用的地址），如果有第二个返回值那么它必须是 error 类型。
func IsConstructor(t reflect.Type) bool {
	returnError := t.NumOut() == 2 && IsErrorType(t.Out(1))
	return IsFuncType(t) && (t.NumOut() == 1 || returnError)
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
	if t.NumIn() < 1 {
		return false
	}
	t0 := t.In(0)
	if t0.Kind() != reflect.Interface {
		return t0 == receiver
	}
	return receiver.Implements(t0)
}

// IsStructPtr 返回是否是结构体的指针类型。
func IsStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}

// TypeName 返回原始类型的全限定名，类型的全限定名用于严格区分相同名称的 bean 对象。
// 类型的全限定名是指包的全路径加上类型名称，例如，gs 包里面的 Container 类型，它的
// 类型全限定名是 github.com/go-spring/spring-core/gs/gs.Container。因为 go
// 语言允许在不同的路径下存在名称相同的包，所以有可能出现(简单)类型名称相同、实例名称
// 相同的但实际上类型不相同的 bean 对象，因此有类型的全限定名这样的概念，用以严格区分
// 同名的 bean 对象。
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

	for { // 去掉指针和数组的包装，以获得原始类型
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
	return typ.String() // 内置类型的路径为空
}

// IsPrimitiveValueType returns whether the input type is the primitive value
// type which only is int, unit, float, bool, string and complex.
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

// IsValueType returns whether the input type is the value type which is the
// primitive value type and their one dimensional composite type including array,
// slice, map and struct, such as [3]string, []string, []int, map[int]int, etc.
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

// IsBeanType 返回是否是 bean 类型。在 go-spring 里，变量的类型分为三种: bean 类
// 型、value(值) 类型以及其他类型。如果一个变量赋值给另一个变量后二者指向相同的内存地
// 址，则称这个变量的类型为 bean 类型，反之则称为 value(值) 类型。根据这个定义，只有
// ptr、interface、chan、func 这四种类型是 bean 类型。
// 可能有人会问，上述四种类型的集合类型如 []interface、map[string]*struct 等也是
// bean 类型吗？根据 go-spring 的定义，它们不是合法的 bean 类型，但是它们是合法的
// bean receiver 类型。那为什么没有把他们也定义为 bean 类型呢？因为如果是切片类型，
// 那么可以转换为注册切片的元素同时加上 order 排序，如果是 map 类型，那么很显然可以转
// 换为依次注册 map 的元素。
// 另外，ptr 一般指一层指针，因为多层指针在 web 开发中很少用到，甚至应该在纯业务代码中
// 禁止使用多层指针。
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

// IsBeanReceiver 返回是否是合法的 bean receiver 类型。顾名思义，bean receiver
// 类型就是指可以保存 bean 地址的类型。除了 ptr、interface、chan、func 四种单体类
// 型可以承载对应的 bean 类型之外，它们的集合类型，即 map、slice、array 类型，也是
// 合法的 bean receiver 类型。它们应用于以单体方式注册 bean 然后以集合方式收集 bean
// 的场景。
func IsBeanReceiver(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return IsBeanType(t.Elem())
	default:
		return IsBeanType(t)
	}
}
