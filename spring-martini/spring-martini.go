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

package SpringMartini

import (
	"context"
	"net/http"
	"github.com/go-martini/martini"
	"github.com/didichuxing/go-spring/spring-web"
)

//
// 容器
//
type MartiniContainer struct {
	HttpServer    *http.Server
	MartiniServer *martini.ClassicMartini
}

func NewMartiniContainer() *MartiniContainer {
	return &MartiniContainer{
		MartiniServer: martini.Classic(),
	}
}

func (container *MartiniContainer) Stop() {
	container.HttpServer.Shutdown(context.TODO())
}

func (container *MartiniContainer) Router(path string) *SpringWeb.WebRouter {
	return SpringWeb.NewWebRouter(container, path)
}

func (container *MartiniContainer) Start(address string) error {
	container.HttpServer = &http.Server{Addr: address, Handler: container.MartiniServer}
	return container.HttpServer.ListenAndServe()
}

func (container *MartiniContainer) StartTLS(address string, certFile, keyFile string) error {
	container.HttpServer = &http.Server{Addr: address, Handler: container.MartiniServer}
	return container.HttpServer.ListenAndServeTLS(certFile, keyFile)
}

func (container *MartiniContainer) GET(path string, fn SpringWeb.Handler, tags ...string) {
	container.MartiniServer.Get(path, (&SpringWeb.HandlerWrapper{fn}).Handler)
}

func (container *MartiniContainer) POST(path string, fn SpringWeb.Handler, tags ...string) {
	container.MartiniServer.Post(path, (&SpringWeb.HandlerWrapper{fn}).Handler)
}
