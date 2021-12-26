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

package gs

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-spring/spring-core/web"
)

func init() {
	gInits = append(gInits, func(s *startup) {
		if s.web {
			Object(new(WebStarter)).Export((*AppEvent)(nil))
		}
	})
}

// WebStarter Web 服务器启动器
type WebStarter struct {
	Containers []web.Container `autowire:""`
	Filters    []web.Filter    `autowire:"${web.server.filters:=*?}"`
	Router     web.Router      `autowire:""`
}

// OnAppStart 应用程序启动事件。
func (starter *WebStarter) OnAppStart(ctx Context) {
	for _, c := range starter.Containers {
		c.AddFilter(starter.Filters...)
	}
	for _, m := range starter.Router.Mappers() {
		for _, c := range starter.getContainers(m) {
			c.AddMapper(web.NewMapper(m.Method(), m.Path(), m.Handler()))
		}
	}
	starter.startContainers(ctx)
}

func (starter *WebStarter) getContainers(mapper *web.Mapper) []web.Container {
	var ret []web.Container
	for _, c := range starter.Containers {
		if strings.HasPrefix(mapper.Path(), c.Config().BasePath) {
			ret = append(ret, c)
		}
	}
	return ret
}

func (starter *WebStarter) startContainers(ctx Context) {
	for i := range starter.Containers {
		c := starter.Containers[i]
		ctx.Go(func(_ context.Context) {
			if err := c.Start(); err != nil && err != http.ErrServerClosed {
				ShutDown(err.Error())
			}
		})
	}
}

// OnAppStop 应用程序结束事件。
func (starter *WebStarter) OnAppStop(ctx context.Context) {
	for _, c := range starter.Containers {
		_ = c.Stop(ctx)
	}
}
