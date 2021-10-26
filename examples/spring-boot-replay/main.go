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

	"github.com/go-spring/spring-base/knife"
	"github.com/go-spring/spring-base/replayer"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/redis"
	"github.com/go-spring/spring-core/web"
	_ "github.com/go-spring/starter-echo"
	_ "github.com/go-spring/starter-go-redis"
)

func init() {
	replayer.SetReplayMode()
}

type controller struct {
	RedisClient redis.Client `autowire:""`
}

func (c *controller) index(webCtx web.Context) {
	ctx := knife.New(webCtx.Context())

	sessionID := "54c8fab33dcb4f46899a3a3b70987164"
	err := knife.Set(ctx, replayer.SessionIDKey, sessionID)
	util.Panic(err).When(err != nil)

	_, err = c.RedisClient.Set(ctx, "a", float64(1))
	util.Panic(err).When(err != nil)

	v, err := c.RedisClient.Get(ctx, "a")
	util.Panic(err).When(err != nil)

	fmt.Printf("get redis a=%v\n", v)
}

func main() {
	gs.Object(new(controller)).Init(func(c *controller) {
		gs.GetMapping("/index", c.index)
	})
	fmt.Printf("program exited %v\n", gs.Run())
}
