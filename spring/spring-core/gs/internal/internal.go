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

package internal

import (
	"reflect"

	"github.com/go-spring/spring-core/conf"
)

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

type RefreshArg struct {
	AutoClear bool
}

type RefreshOption func(arg *RefreshArg)

func AutoClear(enable bool) RefreshOption {
	return func(arg *RefreshArg) {
		arg.AutoClear = enable
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
		return isValueType(t.Elem())
	default:
		return false
	}
}

func isValueType(t reflect.Type) bool {
	if t.Kind() == reflect.Struct {
		return true
	}
	return conf.IsPrimitiveValueType(t)
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
