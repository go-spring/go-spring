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

package app

import (
	"errors"
	"time"

	"github.com/go-spring/go-spring-parent/spring-logger"
	"github.com/go-spring/go-spring/spring-boot"
	"github.com/go-spring/go-spring/spring-core"
)

func init() {
	SpringBoot.RegisterBean(new(MyRunner))
}

type MyRunner struct {
	_ SpringBoot.CommandLineRunner `export:""`
}

func (_ *MyRunner) Run(appCtx SpringBoot.ApplicationContext) {

	appCtx.SafeGoroutine(func() {
		SpringLogger.Trace("get all properties:")
		for k, v := range appCtx.GetProperties() {
			SpringLogger.Tracef("%v=%v", k, v)
		}
		SpringLogger.Info("exit right now in MyRunner::Run")
	})

	fn := func(ctx SpringBoot.ApplicationContext, version string) {
		if version != "v0.0.1" {
			panic(errors.New("error"))
		}
	}
	_ = appCtx.Run(fn, "1:${version:=v0.0.1}").On(SpringCore.ConditionOnProfile("test"))

	appCtx.SafeGoroutine(func() {
		defer SpringLogger.Info("exit after waiting in MyRunner::Run")

		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-appCtx.Context().Done():
				return
			case <-ticker.C:
				SpringLogger.Info("MyRunner::Run")
			}
		}
	})
}
