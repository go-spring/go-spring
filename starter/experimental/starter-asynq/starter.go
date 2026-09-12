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

package StarterAsynq

import (
	"context"

	"github.com/hibiken/asynq"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-asynq/health"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one Asynq instance per entry under "${spring.asynq}". The
	// producer Client bean is always wired; the worker Server bean is
	// additionally wired when server.enabled is true (default off — a
	// long-running worker is an opt-in, per the starter conventions).
	gs.Module(gs.OnProperty("spring.asynq.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.asynq.instances}", func(name string, c Config) error {
			// The Driver bean is selected by the entry's ${driver} key: unset →
			// "?" (nullable by-type — injects the single Driver bean when one
			// is provided, nil otherwise, and the ctor falls back to the
			// bundled DefaultDriver); set → that bean name, and naming a bean
			// that does not exist fails loud. Client, Server and the health
			// indicator share the one Driver bean, so every role dials through
			// the same assembly.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.asynq.instances."+name+".driver:=${spring.asynq.default.driver:=?}}")),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)

			if c.Server.Enabled {
				r.Provide(newServer,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.asynq.instances."+name+".driver:=${spring.asynq.default.driver:=?}}")),
				).Name(name + ":server").Init((*Server).Init).Destroy((*Server).Destroy).
					Export(gs.As[gs.Server]()).Caller(1)
			}

			// Health indicator probes Redis via a fresh inspector round trip.
			r.Provide(func(d Driver) (*health.Indicator, error) {
				// No company Driver bean → fall back to the bundled default.
				if d == nil {
					d = DefaultDriver{}
				}
				connOpt, err := d.RedisConnOpt(context.Background(), c)
				if err != nil {
					return nil, err
				}
				return health2.NewClientHealth(name, connOpt), nil
			}, gs.IndexArg(0, gs.TagArg("${spring.asynq.instances."+name+".driver:=${spring.asynq.default.driver:=?}}"))).Name("asynq:" + name).Caller(1)
			return nil
		})
	})
}

// newClient builds the producer Client bean through the supplied Driver.
func newClient(ctx *gs.ContextProvider, c Config, d Driver) (*Client, error) {
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	connOpt, err := d.RedisConnOpt(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	cl := asynq.NewClient(connOpt)
	return &Client{Client: cl, cfg: c}, nil
}

// newServer builds the worker Server bean, holding the resolved Driver so
// Server.Init can build the RedisConnOpt when it constructs the asynq server.
func newServer(ctx *gs.ContextProvider, c Config, d Driver) (*Server, error) {
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	return &Server{cfg: c, driver: d}, nil
}

var _ = context.Background
