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

package SpringCore_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
)

func TestRunner_Run(t *testing.T) {

	t.Run("not run before AutoWireBeans", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")

		run := false
		SpringCore.OnProfile("dev").WhenMatches(ctx).Run(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
			run = true
		}, "1:${version}")

		assert.Equal(t, run, false)

		ctx.AutoWireBeans()
	})

	t.Run("not run after AutoWireBeans", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.AutoWireBeans()

		run := false
		SpringCore.OnProfile("dev").WhenMatches(ctx).Run(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
			run = true
		}, "1:${version}")

		assert.Equal(t, run, false)
	})

	t.Run("run before AutoWireBeans", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.SetProfile("dev")

		assert.Panic(t, func() {
			run := false
			SpringCore.OnProfile("dev").WhenMatches(ctx).Run(func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
				run = true
			}, "1:${version}")
			assert.Equal(t, run, true)
		}, "should call after ctx.AutoWireBeans()")

		ctx.AutoWireBeans()
	})

	t.Run("run after AutoWireBeans", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.SetProfile("dev")
		ctx.AutoWireBeans()

		run := false
		SpringCore.OnProfile("dev").WhenMatches(ctx).Run(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
			run = true
		}, "1:${version}")

		assert.Equal(t, run, true)
	})
}
