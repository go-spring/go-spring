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
	"github.com/go-spring/spring-core/util"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gs.Object(new(Starter)).Name("starter").Export(gs.AppEvent)
}

// Starter Web 服务器启动器
type Starter struct {
	Containers []web.Container `autowire:""`
}

// OnStartApp 应用程序启动事件。
func (starter *Starter) OnStartApp(ctx gs.AppContext) {

	starter.sortContainers()

	var webFilters struct {
		Filters []web.Filter `autowire:"${web.server.filters}"`
	}

	_, err := ctx.Wire(&webFilters)
	util.Panic(err).When(err != nil)

	for _, c := range starter.Containers {
		c.AddFilter(webFilters.Filters...)
	}

	var router web.Router
	err = ctx.Get(&router)
	util.Panic(err).When(err != nil)

	for _, m := range router.Mappers() {
		if c := starter.getContainer(m); c != nil {
			c.AddMapper(web.NewMapper(m.Method(), m.Path(), m.Handler()))
		}
	}

	starter.startContainers(ctx)
}

// OnStopApp 应用程序结束事件。
func (starter *Starter) OnStopApp(ctx gs.AppContext) {
	for _, c := range starter.Containers {
		_ = c.Stop(context.Background())
	}
}

// sortContainers 按照 BasePath 的前缀关系对容器进行排序。
func (starter *Starter) sortContainers() {
	sort.Slice(starter.Containers, func(i, j int) bool {
		si := starter.Containers[i].Config().BasePath
		sj := starter.Containers[j].Config().BasePath
		if strings.HasPrefix(si, sj) {
			return true
		}
		return strings.Compare(si, sj) >= 0
	})
}

func (starter *Starter) getContainer(mapper *web.Mapper) web.Container {
	for _, c := range starter.Containers {
		if strings.HasPrefix(mapper.Path(), c.Config().BasePath) {
			return c
		}
	}
	return nil
}

func (starter *Starter) startContainers(ctx gs.AppContext) {
	for _, container := range starter.Containers {
		c := container
		ctx.Go(func(_ context.Context) {
			if err := c.Start(); err != nil && err != http.ErrServerClosed {
				gs.ShutDown(err)
			}
		})
	}
}
