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

// Package lib 里面的类型原则上都是只被 gs 及其子包使用的。
package lib

import (
	"context"
	"reflect"

	"github.com/go-spring/spring-core/conf"
)

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

// AppContext 封装 IoC 容器的 context.Context 对象。
type AppContext interface {
	Context() context.Context
	Go(fn func(ctx context.Context))
}

// AppRunner 命令行启动器接口
type AppRunner interface {
	Run(ctx AppContext)
}

// AppEvent 应用运行过程中的事件
type AppEvent interface {
	OnStopApp(ctx AppContext)  // 应用停止的事件
	OnStartApp(ctx AppContext) // 应用启动的事件
}

// Environment 提供获取环境变量和命令行参数的方法，命令行参数优先级更高。
type Environment interface {
	ActiveProfile() string
	ConfigLocations() []string
	ConfigExtensions() []string
	Prop(key string, opts ...conf.GetOption) interface{}
}

type PropertySource interface {
	Load(e Environment) (map[string]*conf.Properties, error)
}
