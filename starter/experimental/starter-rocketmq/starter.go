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

// starter.go is the gs registration + glue concept of this starter: it declares
// the infra log tag and registers the per-instance RocketMQ client group under
// "${spring.rocketmq}", wiring each Config entry to newClient (the dispatch +
// probe) and the Client wrapper's Close (the lifecycle in client.go).

package StarterRocketmq

import (
	"go-spring.org/cloud"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {

	// Register multiple RocketMQ clients as a group.
	// Each instance is created according to the configuration in "${spring.rocketmq}".
	// This allows defining multiple RocketMQ clients dynamically.
	gs.Module(gs.OnProperty("spring.rocketmq.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.rocketmq.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.rocketmq.instances", "spring.rocketmq.default")
		return conf.BindEach(p, "${spring.rocketmq.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.rocketmq.instances."+name+".driver:=${spring.rocketmq.default.driver:=?}}")),
				// The governance center is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy((*Client).Close).Caller(1)

			// Export the broker-neutral messaging.Driver over this client as a bean,
			// so consumers (starter-outbox-gorm, app pub/sub) autowire it like any
			// client bean. It shares the connection's bean name; beans are keyed by
			// (name, type), so it stays distinct from the raw *Client bean.
			// The load-test convention bean is a NULLABLE injection (index 1):
			// present when the application provides one, absent otherwise, and
			// NewDriver falls back to the canonical convention.
			r.Provide(func(cl *Client, prop traffic.Propagator) messaging.Driver {
				return NewDriver(cl, prop)
			}, gs.TagArg(name), gs.IndexArg(1, gs.TagArg("?"))).Name(name).Caller(1)
			return nil
		})
	})
}

// newClient creates a RocketMQ client by dispatching to the (optional) Driver
// bean, which owns full client assembly (name server resolution, credentials,
// the rlog bridge). The Driver returns the client COMPLETE — identity and the
// governance executor are both applied while it is built (see [NewClient]) — and
// only then is the client probed (when Ping is enabled) so a wrong name
// server list fails fast at startup instead of surfacing on the first
// produce/consume. A failed probe abandons the client and releases what was just
// assembled.
//
// mgr and inj are the authority beans the owning packages register; the ctor
// bundles them into the [cloud.ClientParams] it hands the driver, which passes
// it to [NewClient] — so the client is assembled complete in one step, with the
// zero bundle degrading to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, center *governance.Center) (*Client, error) {
	// The client's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.Strings("name_servers", c.NameServers))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.Bool("ping", c.Ping),
			log.Msg("creating rocketmq client"),
		}
	})

	if (c.AccessKey == "") != (c.SecretKey == "") {
		return nil, errutil.Explain(nil, "rocketmq access-key and secret-key must be set together (client %s)", name)
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		return nil, err
	}

	// Fail fast (opt-in): probe the name server list directly on the wire, not
	// through the client, so it is a connectivity check rather than business
	// traffic. A failure abandons the client, so release what was just applied.
	if c.Ping {
		if err = probeNameServer(c.NameServers); err != nil {
			log.Errorf(cctx, log.TagAppDef, err, "rocketmq: ping failed")
			_ = cl.Close()
			return nil, errutil.Explain(err, "rocketmq name server probe failed on %v", c.NameServers)
		}
	}
	log.Infof(cctx, log.TagAppDef, "init rocketmq client success")
	return cl, nil
}
