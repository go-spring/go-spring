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

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gs.Object(new(WebServerStarter)).
		WithName("web-server-starter").
		Export((*gs.ApplicationEvent)(nil))
}

// WebServerStarter Web 服务器启动器
type WebServerStarter struct {
	Containers []web.Container `autowire:"[]?"`
}

// SortContainers 按照 BasePath 的前缀关系对容器进行排序，比如
// "/c/d", "/a/b", "/c", "/a", "/"
// 排序之后是
// "/c/d", "/c", "/a/b", "/a", "/"
func SortContainers(containers []web.Container) {
	sort.Slice(containers, func(i, j int) bool {
		si := containers[i].Config().BasePath
		sj := containers[j].Config().BasePath
		if strings.HasPrefix(si, sj) {
			return true
		}
		return strings.Compare(si, sj) >= 0
	})
}

func (starter *WebServerStarter) OnStartApplication(ctx gs.ApplicationContext) {

	SortContainers(starter.Containers)

	var r *gs.RootRouter
	if err := ctx.Get(&r); err != nil {
		panic(err)
	}

	r.ForEach(func(s string, mapper *web.Mapper) {
		for _, c := range starter.Containers {
			if strings.HasPrefix(mapper.Path(), c.Config().BasePath) {
				c.AddMapper(mapper)
			}
		}
	})

	for _, c := range starter.Containers {
		ctx.Go(func(_ context.Context) {
			if err := c.Start(); err != nil && err != http.ErrServerClosed {
				gs.ShutDown()
			}
		})
	}
}

func (starter *WebServerStarter) OnStopApplication(ctx gs.ApplicationContext) {
	for _, c := range starter.Containers {
		_ = c.Stop(context.Background())
	}
}
