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

//go:generate mockgen -build_flags="-mod=mod" -package=util -source=gs.go -destination=gs_mock.go

package util

import "reflect"

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
		et := t.Elem()
		return et.Kind() == reflect.Struct || IsPrimitiveValueType(et)
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
