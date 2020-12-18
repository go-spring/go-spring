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

	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

type fnArg struct {
	f float32
}

type fnOption func(arg *fnArg)

func TestRunner_Run(t *testing.T) {

	t.Run("before AutoWireBeans", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")

		SpringUtils.AssertPanic(t, func() {
			run := false
			cond := SpringCore.ConditionOnProfile("dev")
			_ = ctx.Run(func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
				run = true
			}, "1:${version}").When(cond.Matches(ctx))
			SpringUtils.AssertEqual(t, run, false)
		}, "should call after AutoWireBeans")

		ctx.AutoWireBeans()
	})

	t.Run("not run", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.AutoWireBeans()

		run := false
		cond := SpringCore.ConditionOnProfile("dev")
		_ = ctx.Run(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
			run = true
		}, "1:${version}").When(cond.Matches(ctx))
		SpringUtils.AssertEqual(t, run, false)
	})

	t.Run("run", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.SetProfile("dev")
		ctx.AutoWireBeans()

		cond := SpringCore.ConditionOnProfile("dev")

		run := false
		fn := func(i *int, version string, options ...fnOption) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
			run = true

			arg := &fnArg{}
			for _, opt := range options {
				opt(arg)
			}
			fmt.Println(arg.f)
		}

		// run When
		{
			_ = ctx.Run(fn, "1:${version}").Options(
				SpringCore.NewOptionArg(func(version string) fnOption {
					return func(arg *fnArg) {
						arg.f = 3.0
					}
				}, "0:${version}")).When(cond.Matches(ctx))
			SpringUtils.AssertEqual(t, run, true)
		}

		// run On
		{
			run = false
			_ = ctx.Run(fn, "1:${version}").On(cond)
			SpringUtils.AssertEqual(t, run, true)
		}
	})
}
