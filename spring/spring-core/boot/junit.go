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

package boot

import (
	"testing"
	"time"

	"github.com/go-spring/spring-core/app"
	"github.com/go-spring/spring-core/core"
)

// JUnitSuite 测试用例集接口
type JUnitSuite interface {
	Test(t *testing.T)
}

// JUnitRunner 测试集执行器
type JUnitRunner struct {
	_ app.CommandLineRunner `export:""`

	Suites  []JUnitSuite `autowire:"[]?"`
	t       *testing.T
	waiting time.Duration
}

func (r *JUnitRunner) Run(ctx core.ApplicationContext) {
	ctx.Go(func() {
		time.Sleep(r.waiting)
		for _, suite := range r.Suites {
			suite.Test(r.t)
		}
		app.ShutDown()
	})
}

// RunTestApplication 启动测试程序，waiting 是测试用例开始前的等待时间，因为不知道程序启动器何时完成
func RunTestApplication(t *testing.T, waiting time.Duration, configLocation ...string) {
	app.ObjBean(&JUnitRunner{t: t, waiting: waiting})
	app.Run(configLocation...)
}
