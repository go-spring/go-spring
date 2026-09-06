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

// starter.go is the gs registration + glue concept: it binds each
// ${spring.nats} entry to a *Conn bean (built by newConn in driver.go) and its
// destroy callback (destroyConn in client.go).
package StarterNats

import (
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple NATS connections as a group.
	// Each instance is created according to the configuration in
	// "${spring.nats}", allowing multiple connections dynamically.
	gs.Module(gs.OnProperty("spring.nats"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.nats}", func(name string, c Config) error {
			r.Provide(newConn,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("?")),
			).Name(name).Destroy(destroyConn).Caller(1)
			return nil
		})
	})
}
