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

package core_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/core"
)

func TestRunner_Run(t *testing.T) {

	t.Run("before AutoWireBeans", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")

		assert.Panic(t, func() {
			_ = ctx.Invoke(func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
			}, "", "${version}")
		}, "should call after AutoWireBeans")

		ctx.AutoWireBeans()
	})

	t.Run("not run", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.AutoWireBeans()

		_ = ctx.Invoke(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
		}, "", "${version}")
	})

	t.Run("run", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.SetProfile("dev")
		ctx.AutoWireBeans()

		fn := func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
		}

		_ = ctx.Invoke(fn, "", "${version}")
	})
}
