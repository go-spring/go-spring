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

package EchoStarter

import (
	"github.com/go-spring/go-spring-web/spring-echo"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/starter-web"
)

func init() {
	SpringBoot.RegisterBeanFn(NewEchoWebServer, "${}")
}

// NewEchoWebServer 创建 echo 适配的 Web 服务器
func NewEchoWebServer(config WebStarter.WebServerConfig) *SpringWeb.WebServer {
	webServer := SpringWeb.NewWebServer()

	if config.EnableHTTP {
		e := SpringEcho.NewContainer()
		e.SetPort(config.Port)
		webServer.AddWebContainer(e)
	}

	if config.EnableHTTPS {
		e := SpringEcho.NewContainer()
		e.SetPort(config.SSLPort)
		e.SetKeyFile(config.SSLKey)
		e.SetCertFile(config.SSLCert)
		webServer.AddWebContainer(e)
	}

	return webServer
}
