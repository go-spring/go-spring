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

package main

import (
	"os"

	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/api"
	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/app"
	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/extension"
	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/filter"
	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/mock"
	_ "github.com/go-spring/go-spring/examples/spring-boot-demo/server"
	"github.com/go-spring/go-spring/spring-boot"
	_ "github.com/go-spring/go-spring/starter-go-redis"
	_ "github.com/go-spring/go-spring/starter-mysql-gorm"
)

func init() {
	// SpringLogger.SetLogger(&SpringLogger.Console{})
}

func main() {

	// 配置文件里面也指定了 spring.profile 的值
	// _ = os.Setenv(SpringBoot.SpringProfile, "test")

	// 过滤系统环境变量
	SpringBoot.ExpectSysProperties("GOPATH")

	// 设置过滤器是否启用
	SpringBoot.SetProperty("key_auth", false)

	SpringBoot.SetProperty("db.url", "root:root@/information_schema?charset=utf8&parseTime=True&loc=Local")

	dir, _ := os.Getwd()
	SpringBoot.SetProperty("static.root", dir)

	configLocations := []string{
		"config/", "k8s:config/config-map.yaml",
	}

	// 等效 SpringBoot.RunApplication(configLocations...)
	SpringBoot.NewApplication().Run(configLocations...)
}
