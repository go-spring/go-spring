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

package SpringUtils

import (
	"sync"
)

// WaitGroup 封装 sync.WaitGroup，提供更简单的 API
type WaitGroup struct {
	wg sync.WaitGroup
}

// Add 添加一个非阻塞的任务，任务在新的 Go 程执行
func (wg *WaitGroup) Add(fn func()) {
	wg.wg.Add(1)
	go func() {
		defer wg.wg.Done()
		fn()
	}()
}

// Wait 等待所有任务执行完成
func (wg *WaitGroup) Wait() {
	wg.wg.Wait()
}
