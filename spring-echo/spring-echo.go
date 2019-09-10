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
	"github.com/didichuxing/go-spring/spring-web"
)

//
// 容器
//
type EchoContainer struct {
	EchoServer *echo.Echo
}

func NewEchoContainer() *EchoContainer {

	server := echo.New()
	server.HidePort = true
	server.HideBanner = true

	server.Use(EchoLogger())

	return &EchoContainer{EchoServer: server,}
}

func (container *EchoContainer) Start(address string) error {
	return container.EchoServer.Start(address)
}

func (container *EchoContainer) StartTLS(address string, certFile, keyFile string) error {
	return container.EchoServer.StartTLS(address, certFile, keyFile)
}

func (container *EchoContainer) Stop() {
	container.EchoServer.Shutdown(context.TODO())
}

func (container *EchoContainer) Router(path string) *SpringWeb.WebRouter {
	return SpringWeb.NewWebRouter(container, path)
}

func (container *EchoContainer) GET(path string, fn SpringWeb.Handler, tags ... string) {
	container.EchoServer.GET(path, NewEchoHandlerWrapper(fn).Handler)
}

func (container *EchoContainer) POST(path string, fn SpringWeb.Handler, tags ... string) {
	container.EchoServer.POST(path, NewEchoHandlerWrapper(fn).Handler)
}

//
// 包装处理器
//
type EchoHandlerWrapper struct {
	SpringWeb.HandlerWrapper
}

func NewEchoHandlerWrapper(fn SpringWeb.Handler) *EchoHandlerWrapper {
	handler := new(EchoHandlerWrapper)
	handler.Fn = fn
	return handler
}

func (handler *EchoHandlerWrapper) Handler(context echo.Context) error {
	r := context.Request()
	w := context.Response().Writer
	handler.HandlerWrapper.Handler(w, r)
	return nil
}
