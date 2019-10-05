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

package SpringHttp

import (
	"context"
	"fmt"
	"net/http"
	"github.com/didi/go-spring/spring-web"
)

type Container struct {
	HttpServer *http.Server
}

func NewContainer() *Container {
	return &Container{}
}

func (c *Container) Stop() {
	c.HttpServer.Shutdown(context.TODO())
}

func (c *Container) Start(address string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: nil}
	return c.HttpServer.ListenAndServe()
}

func (c *Container) StartTLS(address string, certFile, keyFile string) error {
	c.HttpServer = &http.Server{Addr: address, Handler: nil}
	return c.HttpServer.ListenAndServeTLS(certFile, keyFile)
}

func (c *Container) GET(path string, fn SpringWeb.Handler) {
	http.HandleFunc(path, HandlerWrapper(fn))
}

func (c *Container) POST(path string, fn SpringWeb.Handler) {
	http.HandleFunc(path, HandlerWrapper(fn))
}

//
// 处理函数包装器
//
func HandlerWrapper(fn SpringWeb.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {

		webCtx := &Context{
			W: w,
			R: r,
		}

		defer func() {
			if r := recover(); r != nil {
				err, ok := r.(error)
				if !ok {
					err = fmt.Errorf("%v", r)
				}
				webCtx.Error(err)
			}
		}()

		fn(webCtx)
	}
}
