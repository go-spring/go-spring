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
	gs.Module(gs.OnProperty("spring.asynq"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.asynq}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)

			if c.Server.Enabled {
				r.Provide(newServer,
					gs.IndexArg(1, gs.ValueArg(c)),
				).Name(name + ":server").Init((*Server).Init).Destroy((*Server).Destroy).
					Export(gs.As[gs.Server]()).Caller(1)
			}

			// Health indicator probes Redis via a fresh inspector round trip.
			connOpt, err := newRedisConnOpt(context.Background(), c)
			if err != nil {
				return err
			}
			r.Provide(func() health.Indicator {
				return health2.NewClientHealth(name, connOpt)
			}).Name("asynq:" + name).Export(gs.As[health.Indicator]()).Caller(1)
			return nil
		})
	})
}

// newClient builds the producer Client bean.
func newClient(ctx *gs.ContextProvider, c Config) (*Client, error) {
	d, err := lookupDriver(c.Driver)
	if err != nil {
		return nil, err
	}
	connOpt, err := d.RedisConnOpt(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	cl := asynq.NewClient(connOpt)
	return &Client{Client: cl, cfg: c}, nil
}

// newServer builds the worker Server bean.
func newServer(ctx *gs.ContextProvider, c Config) (*Server, error) {
	return &Server{cfg: c}, nil
}

var _ = context.Background
