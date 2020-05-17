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
	"errors"
	"reflect"

	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
)

// WebServer SpringWeb.WebServer 的类型描述符
const WebServer = "github.com/go-spring/go-spring-web/spring-web/SpringWeb.WebServer:"

func init() {
	SpringBoot.RegisterNameBean("web-server-starter", new(WebServerStarter))
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
	_ SpringBoot.ApplicationEvent `export:""`

	WebServer *SpringWeb.WebServer `autowire:""`
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	for _, c := range starter.WebServer.Containers() {
		for _, mapping := range SpringBoot.DefaultWebMapping.Mappings {
			ports := mapping.Ports()
			cfg := c.Config()

			// 路由的端口不匹配
			if len(ports) > 0 && SpringUtils.ContainsInt(ports, cfg.Port) < 0 {
				continue
			}

			// 路由的条件不匹配
			if !mapping.CheckCondition(ctx) {
				continue
			}

			var filters []SpringWeb.Filter

			for _, filter := range mapping.Filters() {
				switch wf := filter.(type) {
				case *SpringBoot.WebFilter:
					if wf.CheckCondition(ctx) { // 满足匹配条件
						if f := wf.Filter(); f != nil {
							filters = append(filters, f)
						}
						if b := wf.FilterBean(); b != "" {
							var bf SpringWeb.Filter
							if ok := ctx.GetBeanByName(b, &bf); !ok {
								panic(errors.New("can't get filter " + b))
							}
							filters = append(filters, bf)
						}
					}
				default:
					filters = append(filters, filter)
				}
			}

			var mapper *SpringWeb.Mapper
			switch handler := mapping.Handler().(type) {
			case SpringWeb.Handler:
				mapper = SpringWeb.NewMapper(mapping.Method(), mapping.Path(), handler, filters)
			case *SpringBoot.MethodHandler:
				receiver := reflect.New(handler.Receiver)
				if !ctx.GetBean(receiver.Interface()) {
					panic(errors.New("can't find bean " + handler.Receiver.String()))
				}
				h := SpringWeb.METHOD(receiver.Elem().Interface(), handler.MethodName)
				mapper = SpringWeb.NewMapper(mapping.Method(), mapping.Path(), h, filters)
			}

			c.AddMapper(mapper)
		}
	}

	starter.WebServer.Start()
}

func (starter *WebServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.WebServer.Stop(context.Background())
}
