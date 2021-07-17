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
	"regexp"
	"sort"
	"strings"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
)

func init() {
	gs.Object(new(Starter)).Name("starter").Export(gs.ApplicationEvent)
}

// Starter Web 服务器启动器
type Starter struct {
	Containers []web.Container `autowire:""`
}

// OnStartApplication 应用程序启动事件。
func (starter *Starter) OnStartApplication(ctx gs.ApplicationContext) {

	var router *gs.RootRouter
	if err := ctx.Get(&router); err != nil {
		panic(err)
	}

	var webFilters struct {
		Filters []web.Filter `autowire:"${web.server.filters}"`
	}

	if _, err := ctx.Wire(&webFilters); err != nil {
		panic(err)
	}

	filterMap := make(map[string][]web.Filter)
	for _, filter := range webFilters.Filters {
		var urlPatterns []string
		if p, ok := filter.(interface{ URLPatterns() []string }); ok {
			urlPatterns = p.URLPatterns()
		} else {
			urlPatterns = []string{"/*"}
		}
		for _, pattern := range urlPatterns {
			filterMap[pattern] = append(filterMap[pattern], filter)
		}
	}

	filterPatterns := make(map[*regexp.Regexp][]web.Filter)
	for pattern, filter := range filterMap {
		exp, err := regexp.Compile(pattern)
		if err != nil {
			panic(err)
		}
		filterPatterns[exp] = filter
	}

	sortContainers(starter.Containers)

	getContainer := func(mapper *web.Mapper) web.Container {
		for _, c := range starter.Containers {
			if strings.HasPrefix(mapper.Path(), c.Config().BasePath) {
				return c
			}
		}
		return nil
	}

	getFilters := func(path string) []web.Filter {
		for pattern, filters := range filterPatterns {
			if pattern.MatchString(path) {
				return filters
			}
		}
		return nil
	}

	router.ForEach(func(path string, mapper *web.Mapper) {
		if c := getContainer(mapper); c != nil {
			c.AddMapper(web.NewMapper(
				mapper.Method(),
				mapper.Path(),
				mapper.Handler(),
				getFilters(path),
			))
		}
	})

	for _, c := range starter.Containers {
		ctx.Go(func(_ context.Context) {
			if err := c.Start(); err != nil && err != http.ErrServerClosed {
				gs.ShutDown(err)
			}
		})
	}
}

// OnStopApplication 应用程序结束事件。
func (starter *Starter) OnStopApplication(ctx gs.ApplicationContext) {
	for _, c := range starter.Containers {
		_ = c.Stop(context.Background())
	}
}

// sortContainers 按照 BasePath 的前缀关系对容器进行排序。
func sortContainers(containers []web.Container) {
	sort.Slice(containers, func(i, j int) bool {
		si := containers[i].Config().BasePath
		sj := containers[j].Config().BasePath
		if strings.HasPrefix(si, sj) {
			return true
		}
		return strings.Compare(si, sj) >= 0
	})
}
