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

package SpringEcho

import (
	"context"

	"github.com/didi/go-spring/spring-web"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

//
// 适配 echo 的 Web 容器
//
type Container struct {
	EchoServer *echo.Echo
}

//
// 工厂函数
//
func NewContainer() *Container {
	e := echo.New()

	// 使用错误恢复机制
	e.Use(middleware.Recover())

	return &Container{
		EchoServer: e,
	}
}

func (c *Container) Stop() {
	c.EchoServer.Shutdown(context.TODO())
}

func (c *Container) Start(address string) error {
	return c.EchoServer.Start(address)
}

func (c *Container) StartTLS(address string, certFile, keyFile string) error {
	return c.EchoServer.StartTLS(address, certFile, keyFile)
}

func (c *Container) GET(path string, fn SpringWeb.Handler) {
	c.EchoServer.GET(path, HandlerWrapper(fn))
}

func (c *Container) POST(path string, fn SpringWeb.Handler) {
	c.EchoServer.POST(path, HandlerWrapper(fn))
}

//
// Web 处理函数包装器
//
func HandlerWrapper(fn SpringWeb.Handler) func(echo.Context) error {
	return func(echoCtx echo.Context) error {
		webCtx := &Context{
			EchoContext: echoCtx,
			HandlerFunc: fn,
		}
		fn(webCtx)
		return nil
	}
}
