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
	"errors"
	"fmt"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/redis"
	_ "github.com/go-spring/starter-redigo"
)

type runner struct {
	Client *redis.Client `autowire:""`
}

func (r *runner) Run(ctx gs.Context) {

	_, err := r.Client.Get(ctx.Context(), "nonexisting")
	if !redis.IsErrNil(err) {
		panic(errors.New("should be redis.ErrNil"))
	}

	_, err = r.Client.Set(ctx.Context(), "mykey", "Hello")
	util.Panic(err).When(err != nil)

	v, err := r.Client.Get(ctx.Context(), "mykey")
	util.Panic(err).When(err != nil)
	fmt.Printf("GET mykey=%q\n", v)
	if v != "Hello" {
		panic(errors.New("should be \"Hello\""))
	}

	go gs.ShutDown()
}

func main() {
	gs.Object(&runner{}).Export((*gs.AppRunner)(nil))
	fmt.Printf("program exited %v\n", gs.Web(false).Run())
}
