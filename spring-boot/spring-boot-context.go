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
	"sync"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-core"
)

type GoFunc func()

// ApplicationContext Application 上下文
type ApplicationContext interface {
	SpringCore.SpringContext

	// SafeGoroutine 安全地启动一个 goroutine
	SafeGoroutine(fn GoFunc)

	// Wait 等待所有 goroutine 退出
	Wait()
}

// defaultApplicationContext ApplicationContext 的默认版本
type defaultApplicationContext struct {
	SpringCore.SpringContext

	wg sync.WaitGroup
}

// SafeGoroutine 安全地启动一个 goroutine
func (ctx *defaultApplicationContext) SafeGoroutine(fn GoFunc) {
	ctx.wg.Add(1)
	go func() {
		defer ctx.wg.Done()

		defer func() {
			if err := recover(); err != nil {
				SpringLogger.Error(err)
			}
		}()

		fn()
	}()
}

// Wait 等待所有 goroutine 安全地退出
func (ctx *defaultApplicationContext) Wait() {
	ctx.wg.Wait()
}
