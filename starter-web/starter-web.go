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

package WebStarter

import (
	"context"

	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {
	SpringBoot.RegisterBean(new(WebServerConfig))
	SpringBoot.RegisterBean(new(WebServerStarter))
}

//
// Web 服务器配置
//
type WebServerConfig struct {
	EnableHTTP  bool   `value:"${web.server.enable:=true}"`      // 是否启用 HTTP
	Port        int    `value:"${web.server.port:=8080}"`        // HTTP 端口
	EnableHTTPS bool   `value:"${web.server.ssl.enable:=false}"` // 是否启用 HTTPS
	SSLPort     int    `value:"${web.server.ssl.port:=8443}"`    // SSL 端口
	SSLCert     string `value:"${web.server.ssl.cert:=}"`        // SSL 证书
	SSLKey      string `value:"${web.server.ssl.key:=}"`         // SSL 秘钥
}

//
// Web 容器启动器
//
type WebServerStarter struct {
	Config *WebServerConfig     `autowire:""`
	Server *SpringWeb.WebServer `autowire:"?"`
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	if starter.Server == nil { // 用户没有定制 Web 服务器
		starter.Server = SpringWeb.NewWebServer()

		// 启动 HTTP 容器
		if starter.Config.EnableHTTP {
			c := SpringWeb.WebContainerFactory()
			c.SetPort(starter.Config.Port)
			starter.Server.AddWebContainer(c)
		}

		// 启动 HTTPS 容器
		if starter.Config.EnableHTTPS {
			c := SpringWeb.WebContainerFactory()
			c.SetCertFile(starter.Config.SSLCert)
			c.SetKeyFile(starter.Config.SSLKey)
			c.SetPort(starter.Config.SSLPort)
			starter.Server.AddWebContainer(c)
		}
	}

	for _, c := range starter.Server.Containers {
		for _, mapping := range SpringBoot.UrlMapping {

			mapper := mapping.Mapper
			ports := mapping.Ports()

			matches := func() bool {
				for _, port := range ports {
					for _, port0 := range c.GetPort() {
						if port == port0 { // 端口匹配
							return true
						}
					}
				}
				return false
			}

			if len(ports) == 0 || matches() {
				if mapping.GetResult(ctx) {
					filters := mapper.Filters()
					for _, s := range mapping.FilterNames() {
						var f SpringWeb.Filter
						ctx.GetBeanByName(s, &f)
						filters = append(filters, f)
					}
					c.Mapping(mapper.Method(), mapper.Path(), mapper.Handler(), filters...)
				}
			}
		}
	}

	starter.Server.Start()
}

func (starter *WebServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.Server.Stop(context.TODO())
}
