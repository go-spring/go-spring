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

package SpringGin

import (
	"context"
	"net/http"

	"github.com/didi/go-spring/spring-web"
	"github.com/gin-gonic/gin"
)

//
// 适配 gin 的 Web 容器
//
type Container struct {
	HttpServer *http.Server
	GinEngine  *gin.Engine
}

//
// 工厂函数
//
func NewContainer() *Container {
	gin.SetMode(gin.ReleaseMode)
	e := gin.Default()
	return &Container{
		GinEngine: e,
	}
}

func (c *Container) Stop() {
	c.HttpServer.Shutdown(context.TODO())
}

func (c *Container) Start(address string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: c.GinEngine}
	return c.HttpServer.ListenAndServe()
}

func (c *Container) StartTLS(address string, certFile, keyFile string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: c.GinEngine}
	return c.HttpServer.ListenAndServeTLS(certFile, keyFile)
}

func (c *Container) GET(path string, fn SpringWeb.Handler) {
	c.GinEngine.GET(path, HandlerWrapper(path, fn))
}

func (c *Container) POST(path string, fn SpringWeb.Handler) {
	c.GinEngine.POST(path, HandlerWrapper(path, fn))
}

func (c *Container) PATCH(path string, fn SpringWeb.Handler) {
	c.GinEngine.PATCH(path, HandlerWrapper(path, fn))
}

func (c *Container) PUT(path string, fn SpringWeb.Handler) {
	c.GinEngine.PUT(path, HandlerWrapper(path, fn))
}

func (c *Container) DELETE(path string, fn SpringWeb.Handler) {
	c.GinEngine.DELETE(path, HandlerWrapper(path, fn))
}

func (c *Container) HEAD(path string, fn SpringWeb.Handler) {
	c.GinEngine.HEAD(path, HandlerWrapper(path, fn))
}

func (c *Container) OPTIONS(path string, fn SpringWeb.Handler) {
	c.GinEngine.OPTIONS(path, HandlerWrapper(path, fn))
}

//
// Web 处理函数包装器
//
func HandlerWrapper(path string, fn SpringWeb.Handler) func(*gin.Context) {
	return func(ginCtx *gin.Context) {
		webCtx := &Context{
			GinContext:  ginCtx,
			HandlerPath: path,
			HandlerFunc: fn,
		}
		fn(webCtx)
	}
}
