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

package gsutil

import (
	"context"
	"reflect"
)

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
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Ptr:
		return true
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

// IsPrimitiveValueType 返回是否是原生值类型。首先，什么是值类型？在发生赋值时，如
// 果传递的是数据本身而不是数据的引用，则称这种类型为值类型。那什么是原生值类型？所谓原
// 生值类型是指 golang 定义的 26 种基础类型里面符合值类型定义的类型。罗列下来，就是说
// Bool、Int、Int8、Int16、Int32、Int64、Uint、Uint8、Uint16、Uint32、Uint64、
// Float32、Float64、Complex64、Complex128、String、Struct 这些基础数据类型都
// 是值类型。当然，需要特别说明的是 Struct 类型必须在保证所有字段都是值类型的时候才是
// 值类型，只要有不是值类型的字段就不是值类型。
func IsPrimitiveValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Bool,
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
		reflect.Complex128,
		reflect.String,
		reflect.Struct:
		return true
	default:
		return false
	}
}

// IsValueType 返回是否是 value 类型。除了原生值类型，它们的集合类型也是值类型，但
// 是仅限于一层复合结构，即 []string、map[string]struct 这种，像 [][]string 则
// 不是值类型，map[string]map[string]string 也不是值类型，因为程序开发过程中，配
// 置项应当越明确越好，而多层次嵌套结构显然会造成信息的不明确，因此不能是值类型。
func IsValueType(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Map, reflect.Slice, reflect.Array:
		return IsPrimitiveValueType(t.Elem())
	default:
		return IsPrimitiveValueType(t)
	}
}

// TypeOf 获取任意数据的真实类型。
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

// TypeName 返回原始类型的全限定名，类型的全限定名用于严格区分相同名称的 bean 对象。
// 类型的全限定名是指包的全路径加上类型名称，例如，gs 包里面的 Container 类型，它的
// 类型全限定名是 github.com/go-spring/spring-core/gs/gs.Container。因为 go
// 语言允许在不同的路径下存在名称相同的包，所以有可能出现(简单)类型名称相同、实例名称
// 相同的但实际上类型不相同的 bean 对象，因此有类型的全限定名这样的概念，用以严格区分
// 同名的 bean 对象。
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

// errorType error 的反射类型。
var errorType = reflect.TypeOf((*error)(nil)).Elem()

// contextType context.Context 的反射类型。
var contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

// IsConstructor 返回以函数形式注册 bean 的函数是否合法。一个合法
// 的注册函数需要以下条件：入参可以有任意多个，支持一般形式和 Option
// 形式，返回值只能有一个或者两个，第一个返回值必须是 bean 源，它可以是
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

// IsStructPtr 返回是否是结构体的指针类型。
func IsStructPtr(t reflect.Type) bool {
	return t.Kind() == reflect.Ptr && t.Elem().Kind() == reflect.Struct
}
