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

	"github.com/labstack/echo"
	"github.com/didi/go-spring/spring-web"
	"github.com/labstack/echo/middleware"
)

type Container struct {
	EchoServer *echo.Echo
}

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
	c.EchoServer.GET(path, NewHandlerWrapper(fn))
}

func (c *Container) POST(path string, fn SpringWeb.Handler) {
	c.EchoServer.POST(path, NewHandlerWrapper(fn))
}

//
// 处理函数包装器
//
type HandlerWrapper struct {
	fn SpringWeb.Handler
}

func NewHandlerWrapper(fn SpringWeb.Handler) echo.HandlerFunc {
	return (&HandlerWrapper{fn}).Handler
}

func (wrapper *HandlerWrapper) Handler(echoCtx echo.Context) error {
	webCtx := &Context{
		EchoContext: echoCtx,
	}
	wrapper.fn(webCtx)
	return nil
}
