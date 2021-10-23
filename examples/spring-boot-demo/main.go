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
	"fmt"
	"os"

	_ "github.com/go-spring/examples/spring-boot-demo/api"
	_ "github.com/go-spring/examples/spring-boot-demo/app"
	_ "github.com/go-spring/examples/spring-boot-demo/extension"
	_ "github.com/go-spring/examples/spring-boot-demo/filter"
	_ "github.com/go-spring/examples/spring-boot-demo/mock"
	"github.com/go-spring/spring-core/gs"
	_ "github.com/go-spring/starter-echo"
	_ "github.com/go-spring/starter-go-redis"
	_ "github.com/go-spring/starter-gorm/mysql"
)

func main() {

	// 注意 env 和 property 的优先级，选择合适的方式。
	gs.Setenv("INCLUDE_ENV_PATTERNS", "GOPATH")
	gs.Setenv("GS_SPRING_PROFILES_ACTIVE", "test")
	gs.Setenv("GS_SPRING_CONFIG_LOCATIONS", "config/")
	gs.Setenv("GS_SPRING_CONFIG_EXTENSIONS", ".properties,.prop,.yaml,.yml,.toml,.tml,.ini")

	dir, _ := os.Getwd()
	gs.Property("static.root", dir)
	gs.Property("db.url", "root:root@/information_schema?charset=utf8&parseTime=True&loc=Local")

	fmt.Println(gs.Run())
}
