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

package StarterTdengine

import (
	"context"
	"strings"
	"time"

	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple TDengine clients as a group, one per entry under
	// "${spring.tdengine}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.tdengine.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.tdengine.instances}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so the ctor
			// applies it while building the client (with the injected governance
			// beans) and Destroy tears it down. The Driver bean is
			// selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when one is provided,
			// nil otherwise, and the ctor falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.tdengine.instances."+name+".driver:=${spring.tdengine.default.driver:=?}}")),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(3, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(4, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) *health.Indicator {
				return NewClientHealth(name, w)
			}, gs.TagArg(name)).Name("tdengine:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new TDengine client based on the provided
// configuration. The driver assembles the client complete — governance applied
// while it is built — and only then is the server pinged, so that
// misconfiguration or an unreachable taosAdapter fails fast rather than on
// first use. A failed probe abandons the client.
//
// mgr and inj are the authority beans the owning packages register. The wiring
// injects them NULLABLY, so both are nil in a container without
// the container as well as in a standalone (non-gs) call; the ctor bundles
// them into the [cloud.ClientParams] it hands the driver, and the zero bundle
// degrades to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating tdengine client, dsn-addr=%s", dsnAddr(c.DSN))

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c, cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		return nil, err
	}
	// Fail fast: probe the assembled client with a ping at startup. The probe is
	// [HealthCheck], the single liveness implementation; it goes straight to the
	// raw pool on purpose: it is a connectivity check, not business traffic, so
	// it must not open a span or spend limiter/breaker budget. A failure
	// abandons the client, so release what was just applied.
	pctx, cancel := context.WithTimeout(ctx.Context, 10*time.Second)
	defer cancel()
	if err = HealthCheck(pctx, cl); err != nil {
		_ = cl.Destroy()
		return nil, errutil.Explain(err, "failed to reach tdengine at %s", dsnAddr(c.DSN))
	}
	return cl, nil
}

// HealthCheck reports whether the TDengine instance is reachable. It is a
// thin readiness probe suitable for wiring into a health endpoint, and the
// single place a TDengine liveness check is defined.
func HealthCheck(ctx context.Context, client *Client) error {
	return client.DB.PingContext(ctx)
}

// dsnAddr extracts a display-safe address from the unified DSN, e.g.
// "root:taosdata@ws(127.0.0.1:6041)/power" -> "127.0.0.1:6041".
func dsnAddr(dsn string) string {
	if i := strings.Index(dsn, "("); i >= 0 {
		if j := strings.Index(dsn[i:], ")"); j > 0 {
			return dsn[i+1 : i+j]
		}
	}
	return dsn
}
