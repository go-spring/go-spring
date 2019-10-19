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

package SpringBoot

import (
	"github.com/go-spring/go-spring/spring-core"
)

//
// 定义 SpringBoot 模块初始化函数，未来可能成为接口
//
type ModuleFunc func(SpringCore.SpringContext)

//
// 定义 SpringBoot 模块数组
//
var Modules = make([]ModuleFunc, 0)

//
// 注册 SpringBoot 模块
//
func RegisterModule(fn ModuleFunc) {
	Modules = append(Modules, fn)
}
