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

package SpringHttpRpc

import (
	"net/http"
	"strings"

	"github.com/didi/go-spring/spring-rpc"
	"github.com/didi/go-spring/spring-web"
)

type Container struct {
	WebContainer SpringWeb.WebContainer
}

func (c *Container) Stop() {
	c.WebContainer.Stop()
}

func (c *Container) Start(address string) error {
	return c.WebContainer.Start(address)
}

func (c *Container) StartTLS(address string, certFile, keyFile string) error {
	return c.WebContainer.StartTLS(address, certFile, keyFile)
}

func (c *Container) Register(service string, method string, fn SpringRpc.Handler) {

	var path string

	if strings.HasPrefix(service, "/") {
		path = service
	} else {
		path = "/" + service + "_" + method
	}

	// HTTP RPC 只能使用 POST 方法传输数据
	c.WebContainer.Register("POST", path, func(ctx SpringWeb.WebContext) {

		// HTTP RPC 只能返回 json 格式的数据
		ctx.Header("Content-Type", "application/json")

		err := ctx.JSON(http.StatusOK, fn(ctx))
		if err != nil {
			ctx.Logger("__rpc_out").Error(err)
		}
	})
}
