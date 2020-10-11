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

package StarterWeb

import (
	"context"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
	"github.com/go-spring/spring-web"
)

// 一般来讲，Starter 包里面只允许包含 init 函数，但是 Web 包比较特殊，它必须配
// 合 echo 或 gin 的 Starter 包一起使用才行，所以为了避免显式导入 Web 包采取了
// 通过 echo 或 gin 依赖的方式自动导入该包，其他 Starter 包还是要符合一般原则的。

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

// WebServerStarter Web 服务器启动器
type WebServerStarter struct {
	_ SpringBoot.ApplicationEvent `export:""`

	WebServer  *SpringWeb.WebServer     `autowire:""`
	Containers []SpringWeb.WebContainer `autowire:"[]?"`
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	// 将收集到的 Web 容器赋值给 Web 服务器
	starter.WebServer.AddContainer(starter.Containers...)

	// 处理 WebServer 级别的过滤器，排除不满足条件的过滤器
	starter.WebServer.ResetFilters(starter.resolveFilters(ctx, starter.WebServer.Filters()))

	for _, c := range starter.Containers {

		// 处理 WebContainer 级别的过滤器，排除不满足条件的过滤器
		c.ResetFilters(starter.resolveFilters(ctx, c.GetFilters()))

		for _, mapping := range SpringBoot.DefaultWebMapping.Mappings {

			// 查看路由的端口是否匹配
			ports := mapping.Ports()
			if len(ports) > 0 && SpringUtils.ContainsInt(ports, c.Config().Port) < 0 {
				continue
			}

			// 查看路由的条件是否匹配
			if !mapping.CheckCondition(ctx) {
				continue
			}

			// 处理 Mapping 级别的过滤器，排除不满足条件的过滤器
			filters := starter.resolveFilters(ctx, mapping.Filters())
			mapper := SpringWeb.NewMapper(mapping.Method(), mapping.Path(), mapping.Handler(), filters)
			c.AddMapper(mapper.WithSwagger(mapping.Swagger()))
		}
	}

	// 容器运行过程中自身发生错误的退出程序，尤其像端口占用这种错误
	starter.WebServer.SetErrorCallback(func(err error) {
		SpringBoot.Exit()
	})

	starter.WebServer.Start()
}

func (starter *WebServerStarter) resolveFilters(ctx SpringCore.SpringContext, filters []SpringWeb.Filter) []SpringWeb.Filter {
	var result []SpringWeb.Filter
	for _, filter := range filters {
		switch f := filter.(type) {
		case *SpringBoot.ConditionalWebFilter:
			result = append(result, f.ResolveFilters(ctx)...)
		default:
			result = append(result, f)
		}
	}
	return result
}

func (starter *WebServerStarter) OnStopApplication(ctx SpringBoot.ApplicationContext) {
	starter.WebServer.Stop(context.Background())
}
