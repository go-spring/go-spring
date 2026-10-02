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
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register one Asynq instance per entry under "${spring.asynq}". The
	// producer Client bean is always wired; the worker Server bean is
	// additionally wired when server.enabled is true (default off — a
	// long-running worker is an opt-in, per the starter conventions).
	gs.Module(gs.OnProperty("spring.asynq.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.asynq.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.asynq.instances", "spring.asynq.default")
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
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(3, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(4, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy((*Client).Destroy).Caller(1)

			if c.Server.Enabled {
				r.Provide(newServer,
					gs.IndexArg(1, gs.ValueArg(c)),
					gs.IndexArg(2, gs.TagArg("${spring.asynq.instances."+name+".driver:=${spring.asynq.default.driver:=?}}")),
				).Name(name + ":server").Destroy((*Server).Destroy).
					Export(gs.As[gs.Server]()).Caller(1)
			}

			// Health indicator probes Redis via a fresh inspector round trip,
			// unless the user disabled it (health=false).
			if c.Health {
				r.Provide(func(d Driver) (*health.Indicator, error) {
					// No company Driver bean → fall back to the bundled default.
					if d == nil {
						d = DefaultDriver{}
					}
					connOpt, err := d.RedisConnOpt(context.Background(), c)
					if err != nil {
						return nil, err
					}
					return NewClientHealth(name, connOpt), nil
				}, gs.IndexArg(0, gs.TagArg("${spring.asynq.instances."+name+".driver:=${spring.asynq.default.driver:=?}}"))).Name("asynq:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient builds the producer Client bean through the supplied Driver. The
// client is assembled complete in one step: the Driver resolves the RedisConnOpt,
// and [NewClient] fixes the identity and applies the params bundle it is
// handed (mgr, inj). There is no Init hook — building the Client and
// initializing it are the same act.
//
// mgr is required (blank-imported governance); a standalone, non-gs caller
// passes a fresh manager, which is exactly "governance off". The zero bundle
// degrades to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	connOpt, err := d.RedisConnOpt(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	return NewClient(connOpt, c.Addr, cloud.ClientParams{Resilience: mgr, Fault: inj}), nil
}

// newServer builds the worker Server bean. The asynq server is constructed here
// from the Driver-resolved RedisConnOpt, so there is no Init hook: the bean is
// complete when this ctor returns, and the app's handlers register against the
// lazily-created mux before Run.
func newServer(ctx *gs.ContextProvider, c Config, d Driver) (*Server, error) {
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	connOpt, err := d.RedisConnOpt(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	srv := asynq.NewServer(connOpt, asynq.Config{
		Concurrency:     c.Concurrency,
		Queues:          c.Queues,
		ShutdownTimeout: c.ShutdownTimeout,
		// Errors and panics inside a handler are asynq's to recover and retry;
		// a handler error is reported via ErrorHandler and a panic is recovered
		// by asynq's own guard, so we keep our own reporting out of the
		// hot path.
	})
	return &Server{srv: srv}, nil
}
