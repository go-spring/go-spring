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
	SpringBoot.RegisterNameBean("std-web-server", new(WebServerStarter)).AsInterface((*SpringBoot.ApplicationEvent)(nil))
}

// WebServerConfig Web 服务器配置
type WebServerConfig struct {
	EnableHTTP  bool   `value:"${web.server.enable:=true}"`      // 是否启用 HTTP
	Port        int    `value:"${web.server.port:=8080}"`        // HTTP 端口
	EnableHTTPS bool   `value:"${web.server.ssl.enable:=false}"` // 是否启用 HTTPS
	SSLPort     int    `value:"${web.server.ssl.port:=8443}"`    // SSL 端口
	SSLCert     string `value:"${web.server.ssl.cert:=}"`        // SSL 证书
	SSLKey      string `value:"${web.server.ssl.key:=}"`         // SSL 秘钥
}

// WebServerStarter Web 容器启动器
type WebServerStarter struct {
	WebServer *SpringWeb.WebServer `autowire:""`
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	for _, c := range starter.WebServer.Containers {
		for _, mapping := range SpringBoot.DefaultWebMapping.Mappings {
			if mapping.Port() == 0 || mapping.Port() == c.GetPort() {
				if mapping.Matches(ctx) {
					filters := mapping.Filters()
					for _, s := range mapping.FilterNames() {
						var f SpringWeb.Filter
						ctx.GetBeanByName(s, &f)
						filters = append(filters, f)
					}
					c.Request(mapping.Method(), mapping.Path(), mapping.Handler(), filters...)
				}
			}
		}
	}

	starter.WebServer.Start()
}

func (starter *WebServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.WebServer.Stop(context.Background())
}
