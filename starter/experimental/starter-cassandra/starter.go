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

package StarterCassandra

import (
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Cassandra clients as a group, one per entry under
	// "${spring.cassandra}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.cassandra.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.cassandra.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.cassandra.instances", "spring.cassandra.default")
		return conf.BindEach(p, "${spring.cassandra.instances}", func(name string, c Config) error {
			// The ctor reads the authorities off the injected governance center and
			// hands them to the Driver, which passes them to NewClient — so the client is assembled
			// complete in one step, and Destroy tears the executor down. The
			// Driver bean is selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when one is provided,
			// nil otherwise, and the ctor falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.cassandra.instances."+name+".driver:=${spring.cassandra.default.driver:=?}}")),
				// The governance center is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy((*Client).Close).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. The probe delegates to
			// HealthCheck, which goes straight to the raw session. Skipped when
			// c.Health is false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("cassandra:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new Cassandra client based on the provided
// configuration, wrapped so every statement declares its identity (see
// observe.go) and flows through the governance guard.
//
// center is the governance center — the family's sole injection point. The
// wiring injects it NULLABLY, so it is nil in a container without governance as
// well as in a standalone (non-gs) call; the ctor reads the resilience and fault
// authorities from it and bundles
// them into the [cloud.ClientParams] it hands the driver, which passes it to
// [NewClient] — so the client is assembled complete in one step, with the zero
// bundle degrading to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, center *governance.Center) (*Client, error) {
	log.Debug(ctx.Context, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.Strings("hosts", c.Hosts),
			log.String("keyspace", c.Keyspace),
			log.Msg("creating cassandra client"),
		}
	})

	if (c.Username == "") != (c.Password == "") {
		return nil, errutil.Explain(nil, "cassandra username and password must be set together")
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c, cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		return nil, err
	}
	// The Driver returned the client complete — identity and governance both
	// applied while it was built. There is no Init hook and nothing else runs
	// after this — the bean is complete when this ctor returns.
	// Fail fast: probe the cluster once at startup so misconfiguration or an
	// unreachable cluster surfaces during boot rather than on first use. The
	// probe is [HealthCheck] — the same single health implementation the
	// Actuator indicator uses — which goes straight to the raw session on
	// purpose: it is a connectivity check, not business traffic, so it must not
	// open a span or spend limiter/breaker budget. A failure abandons the
	// client, so release what was just applied. Off by default (c.Ping): a
	// cluster that is not up yet must not block startup.
	if c.Ping {
		if err := HealthCheck(ctx.Context, client); err != nil {
			log.Error(ctx.Context, log.TagAppDef, err, log.Msg("cassandra: startup probe failed"))
			_ = client.Close()
			return nil, errutil.Explain(err, "failed to reach cassandra cluster %v", c.Hosts)
		}
	}
	log.Info(ctx.Context, log.TagAppDef, log.Strings("hosts", c.Hosts), log.Msg("init cassandra client success"))
	return client, nil
}
