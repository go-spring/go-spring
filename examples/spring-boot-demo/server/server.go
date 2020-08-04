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

package server

import (
	"fmt"

	"github.com/go-spring/go-spring-web/spring-echo"
	"github.com/go-spring/go-spring-web/spring-web"
	"github.com/go-spring/go-spring-web/static-filter"
	"github.com/go-spring/go-spring/examples/spring-boot-demo/filter"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/starter-web"
)

func init() {

	// 测试 Config 系列函数的功能
	{
		SpringBoot.ConfigWithName("config_f2", func(filter *filter.NumberFilter) {
			fmt.Println("NumberFilter:", filter.N)
		}, "f2").ConditionOnPropertyValue("f2.enable", true)

		SpringBoot.ConfigWithName("config_f5", func(filter *filter.NumberFilter, appName string) {
			fmt.Println("NumberFilter:", filter.N, "appName:", appName)
		}, "f5", "${spring.application.name}").After("config_f2")

		SpringBoot.Config(func(filter *filter.NumberFilter) {
			fmt.Println("NumberFilter:", filter.N)
		}, "f7").Before("config_f2").ConditionOnPropertyValue("f7.enable", true)
	}

	SpringBoot.RegisterBeanFn(func(config StarterWeb.WebServerConfig) SpringWeb.WebContainer {
		cfg := SpringWeb.ContainerConfig{Port: config.Port}
		c := SpringEcho.NewContainer(cfg)
		c.AddFilter(SpringBoot.FilterBean("container"))
		c.Swagger().WithDescription("spring boot test")
		return c
	})

	SpringBoot.RegisterBeanFn(func() *SpringWeb.WebServer {
		return SpringWeb.NewWebServer().AddFilter(
			SpringBoot.FilterBean("server"),
			SpringBoot.FilterBean((*StaticFilter.StaticFilter)(nil)),
		)
	})

	SpringBoot.RegisterFilterFn(StaticFilter.New, "${static.root:=/}", "${static.url-prefix:=/static}")
}
