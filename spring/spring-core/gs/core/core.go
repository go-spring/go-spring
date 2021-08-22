/*
 * Copyright 2012-2019 the original author or authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by bootlicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package core

import (
	"context"
	"reflect"

	"github.com/go-spring/spring-core/conf"
)

type Properties interface {
	Keys() []string
	Has(key string) bool
	Get(key string, opts ...conf.GetOption) string
	Bind(i interface{}, opts ...conf.BindOption) error
}

// Arg 用于为函数参数提供绑定值。可以是 bean.Selector 类型，表示注入 bean ；
// 可以是 ${X:=Y} 形式的字符串，表示属性绑定或者注入 bean ；可以是 ValueArg
// 类型，表示不从 IoC 容器获取而是用户传入的普通值；可以是 IndexArg 类型，表示
// 带有下标的参数绑定；可以是 *optionArg 类型，用于为 Option 方法提供参数绑定。
type Arg interface{}

// BeanSelector bean 选择器，可以是 bean ID 字符串，可以是 reflect.Type
// 对象，可以是形如 (*error)(nil) 的指针，还可以是 Definition 类型的对象。
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

type BeanRegistry interface {
	Get(i interface{}, selectors ...BeanSelector) error
	Wire(objOrCtor interface{}, ctorArgs ...Arg) (interface{}, error)
	Invoke(fn interface{}, args ...Arg) ([]interface{}, error)
}

// Environment 提供了一些在 IoC 容器启动后基于反射获取和使用 property 与 bean 的接
// 口。因为很多人会担心在运行时大量使用反射会降低程序性能，所以命名为 Environment，取
// 其诱人但危险的含义。事实上，这些在 IoC 容器启动后使用属性绑定和依赖注入的方案，
// 都可以转换为启动阶段的方案以提高程序的性能。
// 另一方面，为了统一 Container 和 App 两种启动方式下这些方法的使用方式，需要提取
// 出一个可共用的接口来，也就是说，无论程序是 Container 方式启动还是 App 方式启动，
// 都可以在需要使用这些方法的地方注入一个 Environment 对象而不是 Container 对象或者
// App 对象，从而实现使用方式的统一。
type Environment interface {
	Properties() Properties
	BeanRegistry() BeanRegistry
	Context() context.Context
	Go(fn func(ctx context.Context))
}
