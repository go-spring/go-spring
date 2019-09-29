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

	SpringWeb "github.com/didi/go-spring/spring-web"
	"github.com/gin-gonic/gin"
)

//
// 容器
//
type GinContainer struct {
	GinRouter *gin.Engine
	GinServer *http.Server
}

func NewGinContainer() *GinContainer {

	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	srv := &http.Server{
		Handler: router,
	}
	return &GinContainer{GinServer: srv, GinRouter: router}
}

func (container *GinContainer) Start(address string) error {
	container.GinServer.Addr = address
	return container.GinServer.ListenAndServe()
}

func (container *GinContainer) StartTLS(address string, certFile, keyFile string) error {
	container.GinServer.Addr = address
	return container.GinServer.ListenAndServeTLS(certFile, keyFile)
}

func (container *GinContainer) Stop() {
	container.GinServer.Shutdown(context.TODO())
}

func (container *GinContainer) Router(path string) *SpringWeb.WebRouter {
	return SpringWeb.NewWebRouter(container, path)
}

func (container *GinContainer) GET(path string, fn SpringWeb.Handler, tags ...string) {
	container.GinRouter.GET(path, NewGinHandlerWrapper(fn).Handler)
}

func (container *GinContainer) POST(path string, fn SpringWeb.Handler, tags ...string) {
	container.GinRouter.POST(path, NewGinHandlerWrapper(fn).Handler)
}

//
// 包装处理器
//
type GinHandlerWrapper struct {
	SpringWeb.HandlerWrapper
}

func NewGinHandlerWrapper(fn SpringWeb.Handler) *GinHandlerWrapper {
	handler := new(GinHandlerWrapper)
	handler.Fn = fn
	return handler
}

func (handler *GinHandlerWrapper) Handler(context *gin.Context) {
	r := context.Request
	w := context.Writer
	handler.HandlerWrapper.Handler(w, r)
}
