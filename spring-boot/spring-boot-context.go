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
	"fmt"
	"sync"

	"github.com/go-spring/go-spring/spring-core"
)

type GoFunc func()

//
// 定义 Application 上下文。
//
type ApplicationContext interface {
	// 继承 SpringContext 的功能。
	SpringCore.SpringContext

	// 安全的启动一个 goroutine。
	SafeGoroutine(fn GoFunc)

	// 等待所有 goroutine 退出。
	Wait()
}

//
// 定义 ApplicationContext 的默认版本。
//
type DefaultApplicationContext struct {
	*SpringCore.DefaultSpringContext

	wg sync.WaitGroup
}

//
// 安全地启动一个 goroutine。
//
func (ctx *DefaultApplicationContext) SafeGoroutine(fn GoFunc) {
	go func() {

		defer func() {
			if err := recover(); err != nil {
				fmt.Println(err)
			}
		}()

		ctx.wg.Add(1)
		defer ctx.wg.Done()

		fn()
	}()
}

//
// 等待所有 goroutine 安全地退出。
//
func (ctx *DefaultApplicationContext) Wait() {
	ctx.wg.Wait()
}
