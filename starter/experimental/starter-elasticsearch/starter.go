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

package StarterElasticsearch

import (
	"context"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/mesh"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Elasticsearch clients as a group, one per entry under
	// "${spring.elasticsearch}". A gs.Module (rather than gs.Group) is used so
	// each instance's *Client bean can be paired with a health.Indicator
	// registered under the same name — and to attach the file:line of this
	// registration to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.elasticsearch.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.elasticsearch.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.elasticsearch.instances", "spring.elasticsearch.default")
		return conf.BindEach(p, "${spring.elasticsearch.instances}", func(name string, c Config) error {
			// The wrapper bean is assembled complete by the ctor: identity and
			// governance are applied while the Driver builds it, and Destroy tears
			// down the executor. The instance's discovery.Discovery backend bean is
			// injected by name from the entry's ${discovery} label (default
			// "default"; optional, so an app with no backend beans at all gets nil
			// here). The Driver bean is selected by the entry's ${driver} key:
			// unset → "?" (nullable by-type — injects the single Driver bean when
			// one is provided, nil otherwise, and the ctor falls back to the
			// bundled DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.elasticsearch.instances."+name+".discovery:=${spring.elasticsearch.default.discovery:=none}}?")),
				gs.IndexArg(3, gs.TagArg("${spring.elasticsearch.instances."+name+".driver:=${spring.elasticsearch.default.driver:=?}}")),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(4, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(5, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. Skipped when c.Health is
			// false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("elasticsearch:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new Elasticsearch client based on the provided
// configuration, wrapped so every request declares its identity on the transport
// chain and then flows through the resilience round-tripper, which emits the one
// span, the duration metrics and the access log. When c.Ping is set the cluster
// is then probed once at startup so that misconfiguration or an unreachable
// cluster fails fast rather than on first use; the probe is off by default, so a
// cluster that is not up yet does not block startup.
//
// When c.ServiceName is set and mesh mode is off, a by-name loader is built
// against backend (the discovery backend the entry's ${discovery} label
// resolved to), its current live endpoint snapshot is turned into
// "scheme://host:port" node addresses, and those override c.Addresses. This read
// is the fail-fast probe and the seed; the live feed is the connection pool
// DefaultDriver installs over the same resolver, so cluster membership keeps
// following the naming service (see pool.go). In mesh mode the sidecar owns
// discovery+LB, so the static Addresses (or CloudID) are used unchanged. See
// Config.ServiceName.
//
// mgr and inj are the governance beans the container injects (both nil in a
// standalone, non-gs call); the ctor bundles them — together with backend —
// into the [cloud.ClientParams] it hands the driver, which passes it to
// [NewClient]: the client is assembled complete in one step, with the zero
// bundle degrading to an observed-only, loudly-unmanaged executor. The wiring
// injects them as REQUIRED, and each is registered by the package that owns
// it — which this starter imports — so "governance off" is
// spring.governance.enabled=false, never an absent bean.
func newClient(ctx *gs.ContextProvider, c Config, backend discovery.Discovery, d Driver,
	mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	if c.ServiceName != "" && !mesh.Enabled() {
		addrs, err := resolveAddresses(ctx.Context, c, backend)
		if err != nil {
			return nil, err
		}
		c.Addresses = addrs
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: mgr, Fault: inj, Discovery: backend})
	if err != nil {
		return nil, errutil.Explain(err, "failed to create elasticsearch client")
	}
	// The Driver returned the client complete — identity, governance and the
	// declaration transport all applied while it was built. There is no Init hook
	// and nothing else runs after this — the bean is complete when this ctor
	// returns.
	// Fail fast: probe the cluster once at startup so a misconfigured or
	// unreachable cluster surfaces during boot rather than on the first request.
	// The probe is [HealthCheck], the single liveness implementation; it goes
	// straight to the raw client on purpose — it is a connectivity check, not
	// business traffic. A failure abandons the client, so release what was just
	// applied. Off by default (c.Ping): a cluster that is not up yet must not
	// block startup.
	if c.Ping {
		if err := HealthCheck(ctx.Context, client); err != nil {
			_ = client.Destroy()
			return nil, errutil.Explain(err, "failed to reach elasticsearch cluster")
		}
	}
	return client, nil
}

// HealthCheck reports whether the Elasticsearch cluster is reachable by issuing
// an Info request. It is a thin readiness probe suitable for wiring into a
// health endpoint, and the single place an Elasticsearch liveness check is
// defined. The probe goes straight to the raw client on purpose: a readiness
// check must reflect the backend, not the wrapper. A context is always passed to
// Info so the probe inherits cancellation and a deadline from its caller.
func HealthCheck(ctx context.Context, client *Client) error {
	res, err := client.Client.Info(client.Client.Info.WithContext(ctx))
	if err != nil {
		return err
	}
	return infoStatus(res)
}

// infoStatus closes an Info response body and turns its HTTP status into an
// error, so the startup probe and the readiness probe read the same way.
func infoStatus(res *esapi.Response) error {
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return errutil.Explain(nil, "elasticsearch: info returned %s", res.Status())
	}
	return nil
}
