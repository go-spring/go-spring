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
	"net/http"
	"github.com/didi/go-spring/spring-web"
)

type Container struct {
	HttpServer *http.Server
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

func (c *Container) Register(method string, path string, fn SpringWeb.Handler) {
	panic(SpringWeb.UNSUPPORTED_METHOD)
}
