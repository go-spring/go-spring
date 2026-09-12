/*
 * Copyright 2025 The Go-Spring Authors.
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

// starter.go is the gs wiring: one executor per entry under
// "${spring.xxljob}". The executor is a gs.Server (its callback HTTP server),
// and it registers/removes itself with the admin on startup/shutdown.
package StarterXxljob

import (
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.xxljob.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.xxljob.instances}", func(name string, c Config) error {
			r.Provide(newExecutor,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
			).Name(name).Export(gs.As[gs.Server]()).Destroy((*Executor).Destroy).Caller(1)

			r.Provide(func(e *Executor) *health.Indicator {
				return e.Health()
			}, gs.TagArg(name)).Name("xxljob:" + name).Caller(1)
			return nil
		})
	})
}
