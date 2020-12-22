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
	"net/http"
	"sort"
	"strings"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-web"
)

func init() {
	SpringBoot.RegisterNameBean("web-server-starter", new(WebServerStarter))
}

// WebServerConfig Web 服务器配置
type WebServerConfig struct {
	Port        int    `value:"${web.server.port:=8080}"`        // HTTP 端口
	EnableHTTPS bool   `value:"${web.server.ssl.enable:=false}"` // 是否启用 HTTPS
	SSLCert     string `value:"${web.server.ssl.cert:=}"`        // SSL 证书
	SSLKey      string `value:"${web.server.ssl.key:=}"`         // SSL 秘钥
}

// WebServerStarter Web 服务器启动器
type WebServerStarter struct {
	_ SpringBoot.ApplicationEvent `export:""`

	Containers []SpringWeb.WebContainer `autowire:"[]?"`
}

// SortContainers 按照 BasePath 的前缀关系对容器进行排序，比如
// "/c/d", "/a/b", "/c", "/a", "/"
// 排序之后是
// "/c/d", "/c", "/a/b", "/a", "/"
func SortContainers(containers []SpringWeb.WebContainer) {
	sort.Slice(containers, func(i, j int) bool {
		si := containers[i].Config().BasePath
		sj := containers[j].Config().BasePath
		if strings.HasPrefix(si, sj) {
			return true
		}
		return strings.Compare(si, sj) >= 0
	})
}

func (starter *WebServerStarter) OnStartApplication(ctx SpringBoot.ApplicationContext) {

	for _, c := range starter.Containers {
		c.SetFilters(starter.resolveFilters(ctx, c.GetFilters()))
	}

	SortContainers(starter.Containers)

	for _, mapping := range SpringBoot.DefaultWebMapping.Mappings {
		if mapping.CheckCondition(ctx) {
			// 选择第一个最合适的路径前缀进行注册
			starter.resolveMapping(ctx, mapping)
		}
	}

	for _, c := range starter.Containers {
		ctx.SafeGoroutine(func() { // 如果端口被占用则退出程序
			if err := c.Start(); err != nil && err != http.ErrServerClosed {
				SpringBoot.Exit()
			}
		})
	}
}

// resolveMapping 选择第一个最合适的路径前缀进行注册
func (starter *WebServerStarter) resolveMapping(ctx SpringCore.SpringContext, mapping *SpringBoot.Mapping) {
	for _, c := range starter.Containers {
		if strings.HasPrefix(mapping.Path(), c.Config().BasePath) {
			filters := starter.resolveFilters(ctx, mapping.Filters())
			mapper := SpringWeb.NewMapper(mapping.Method(), mapping.Path(), mapping.Handler(), filters)
			c.AddMapper(mapper.WithSwagger(mapping.Swagger()))
			return
		}
	}
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
	for _, c := range starter.Containers {
		_ = c.Stop(context.Background())
	}
}
