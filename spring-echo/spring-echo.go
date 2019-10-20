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

	"github.com/go-spring/go-spring/spring-web"
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

func (c *Container) Route(path string, filters ...SpringWeb.Filter) *SpringWeb.Route {
	return SpringWeb.NewRoute(c, path, filters)
}

func (c *Container) Group(path string, fn SpringWeb.GroupHandler, filters ...SpringWeb.Filter) {
	fn(SpringWeb.NewRoute(c, path, filters))
}

func (c *Container) GET(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.GET(path, HandlerWrapper(fn, filters...))
}

func (c *Container) PATCH(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.PATCH(path, HandlerWrapper(fn, filters...))
}

func (c *Container) PUT(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.PUT(path, HandlerWrapper(fn, filters...))
}

func (c *Container) POST(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.POST(path, HandlerWrapper(fn, filters...))
}

func (c *Container) DELETE(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.DELETE(path, HandlerWrapper(fn, filters...))
}

func (c *Container) HEAD(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.HEAD(path, HandlerWrapper(fn, filters...))
}

func (c *Container) OPTIONS(path string, fn SpringWeb.Handler, filters ...SpringWeb.Filter) {
	c.EchoServer.OPTIONS(path, HandlerWrapper(fn, filters...))
}

//
// Web 处理函数包装器
//
func HandlerWrapper(fn SpringWeb.Handler, filters ...SpringWeb.Filter) echo.HandlerFunc {
	return func(echoCtx echo.Context) error {

		webCtx := &Context{
			EchoContext: echoCtx,
			HandlerFunc: fn,
		}

		SpringWeb.InvokeHandler(webCtx, fn, filters)
		return nil
	}
}
