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
)

// Selector Bean 选择器，可以是 BeanId 字符串，可以是 reflect.Type 对
// 象或者形如 (*error)(nil) 的对象指针，还可以是 Definition 类型的对象。
type Selector interface{}

// Definition Bean 元数据定义。
type Definition interface {
	IsWired() bool // 是否注入完成

	Type() reflect.Type     // 类型
	Value() reflect.Value   // 值
	Interface() interface{} // 源

	ID() string       // 返回 Bean 的 ID
	Name() string     // 返回 Bean 的名称
	TypeName() string // 返回类型的全限定名
}
