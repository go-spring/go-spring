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

//
// 实现了一个通用的启动框架。
//
package BootStarter

import (
	"os"
	"os/signal"
	"syscall"
)

//
// 应用执行器。
//
type AppRunner interface {
	Start()    // 启动执行器
	ShutDown() // 关闭执行器
}

var exitChan chan struct{}

//
// 启动应用执行器。
//
func Run(runner AppRunner) {

	exitChan = make(chan struct{})

	go func() {
		// 响应控制台的 Ctrl+C 及 kill 命令。

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

		<-sig

		SafeCloseChan(exitChan)
	}()

	runner.Start()

	<-exitChan

	runner.ShutDown()
}

//
// 退出当前的应用程序。
//
func Exit() {
	SafeCloseChan(exitChan)
}

func SafeCloseChan(ch chan struct{}) {
	select {
	case <-ch:
		// Already closed. Don't close again.
	default:
		// Safe to close here.
		close(ch)
	}
}
