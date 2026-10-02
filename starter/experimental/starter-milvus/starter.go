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

package StarterMilvus

import (
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register Milvus clients as a group, one per entry under
	// "${spring.milvus}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration to
	// the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.milvus.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.milvus.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.milvus.instances", "spring.milvus.default")
		return conf.BindEach(p, "${spring.milvus.instances}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(2, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(3, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy((*Client).Destroy).Caller(1)

			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. Skipped when c.Health is
			// false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("milvus:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient builds the Milvus client — [NewClient] owns the whole assembly,
// guard and governance included — and when c.Ping is set probes it once so a
// wrong address or bad credential fails fast at startup instead of on first
// query; the probe is off by default, so a server that is not up yet does not
// block startup. There is no Init hook: the client is complete when this ctor
// returns.
//
// mgr and inj are the governance beans the container injects; the ctor bundles
// them into the [cloud.ClientParams] it hands [NewClient], so the client is
// assembled complete in one step, with the zero bundle degrading to an
// observed-only, loudly-unmanaged executor.
//
// cp carries the application context gs injects into a constructor (a bare
// context.Context is not an injectable bean).
func newClient(cp *gs.ContextProvider, c Config, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	ctx := cp.Context
	w, err := NewClient(ctx, c, cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		return nil, err
	}
	// Fail-fast probe: HealthCheck lists collections, verifying reachability +
	// auth. The guard is already installed, so the probe runs under it, matching
	// the assembled-then-probed order of the other client starters; it goes
	// straight to the raw client (see [HealthCheck]). On failure the client just
	// assembled is released, not leaked. Off by default (c.Ping): a server that
	// is not up yet must not block startup.
	if c.Ping {
		if err := HealthCheck(ctx, w); err != nil {
			_ = w.Destroy()
			return nil, errutil.Explain(err, "milvus: startup probe failed")
		}
	}
	return w, nil
}
