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

// Package bean 定义了一些和 Bean 相关的类型。
package bean

import (
	"reflect"

	"github.com/go-spring/spring-core/util"
)

// Selector Bean 选择器，可以是 BeanId 字符串，可以是 reflect.Type 对
// 象或者形如 (*error)(nil) 的对象指针，还可以是 Definition 类型的对象。
type Selector interface{}

// Definition Bean 元数据定义。
type Definition interface {
	Type() reflect.Type     // 类型
	Value() reflect.Value   // 值
	Interface() interface{} // 源

	ID() string       // 返回 Bean 的 ID
	Name() string     // 返回 Bean 的名称
	TypeName() string // 返回类型的全限定名

	FileLine() string    // 返回 Bean 的注册点
	Description() string // 返回 Bean 的详细描述
}

// IsConstructor 返回以函数形式注册 Bean 的函数是否合法。一个合法
// 的注册函数需要以下条件：入参可以有任意多个，支持一般形式和 Option
// 形式，返回值只能有一个或者两个，第一个返回值必须是 Bean 源，它可以是
// 结构体等值类型也可以是指针等引用类型，为值类型时内部会自动转换为引用类
// 型（获取可引用的地址），如果有第二个返回值那么它必须是 error 类型。
func IsConstructor(t reflect.Type) bool {
	returnError := t.NumOut() == 2 && util.IsErrorType(t.Out(1))
	return util.IsFuncType(t) && (t.NumOut() == 1 || returnError)
}
