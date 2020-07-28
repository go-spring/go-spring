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
	"fmt"

	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring/spring-boot"
)

func init() {

	SpringBoot.RegisterNameBean("web-server", SpringWeb.NewWebServer()).
		ConditionOnMissingBean((*SpringWeb.WebServer)(nil)).
		ConditionOnOptionalPropertyValue("web-server.enable", true)

	SpringBoot.RegisterNameBean("web-server-starter", new(WebServerStarter)).
		ConditionOnMissingBean((*WebServerStarter)(nil)).
		ConditionOnOptionalPropertyValue("web-server-starter.enable", true)
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

	WebServer  *SpringWeb.WebServer     `autowire:""`
	Containers []SpringWeb.WebContainer `autowire:"[]?"`
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	// 将收集到的 Web 容器赋值给 Web 服务器
	starter.WebServer.AddContainer(starter.Containers...)

	resolveFilters := func(filters []SpringWeb.Filter) (result []SpringWeb.Filter) {
		for _, filter := range filters {
			switch wf := filter.(type) {
			case *SpringBoot.ConditionalWebFilter:
				if wf.CheckCondition(ctx) { // 满足匹配条件
					if f := wf.Filter(); len(f) > 0 {
						result = append(result, f...)
					}
					if b := wf.FilterBean(); len(b) > 0 {
						for _, beanId := range b {
							var bf SpringWeb.Filter
							if !ctx.GetBean(&bf, beanId) {
								panic(fmt.Errorf("can't get filter %v", beanId))
							}
							result = append(result, bf)
						}
					}
				}
			default:
				result = append(result, filter)
			}
		}
		return
	}

	// 处理 WebServer 的过滤器
	{
		filters := starter.WebServer.Filters()
		resolved := resolveFilters(filters)
		starter.WebServer.ResetFilters(resolved)
	}

	// 处理 WebContainer 的过滤器
	{
		for _, container := range starter.Containers {
			filters := container.GetFilters()
			resolved := resolveFilters(filters)
			container.ResetFilters(resolved)
		}
	}

	for _, c := range starter.Containers {
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

			// 处理 Mapping 的过滤器
			filters := mapping.Filters()
			filters = resolveFilters(filters)

			mapper := SpringWeb.NewMapper(mapping.Method(), mapping.Path(), mapping.Handler(), filters)
			c.AddMapper(mapper.WithSwagger(mapping.Swagger()))
		}
	}

	starter.WebServer.Start()
}

func (starter *WebServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.WebServer.Stop(context.Background())
}
