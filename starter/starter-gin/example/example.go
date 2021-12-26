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

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-gin"
)

func init() {
	gs.Object(new(Controller)).Init(func(c *Controller) {
		gs.GetMapping("/", c.Hello)
	})
}

type Controller struct {
	GOPATH string `value:"${GOPATH}"`
}

func (c *Controller) Hello(ctx web.Context) {
	ctx.String("%s - hello world!", c.GOPATH)
}

func main() {
	fmt.Println(gs.Run())
}

// ➜ curl http://localhost:8000/
// /Users/didi/work - hello world!
