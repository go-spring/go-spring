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

package StarterS3

import (
	"context"
	"go-spring.org/cloud/governance"

	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple S3 clients as a group, one per entry under
	// "${spring.s3}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.s3.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.s3.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.s3.instances", "spring.s3.default")
		return conf.BindEach(p, "${spring.s3.instances}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so the ctor builds it
			// from the injected governance center and Destroy tears it down. The
			// Driver bean is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver bean
			// when one is provided, nil otherwise, and the ctor falls back to the
			// bundled DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.s3.instances."+name+".driver:=${spring.s3.default.driver:=?}}")),
				// The governance center is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. Skipped when c.Health is
			// false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("s3:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new S3 client based on the provided configuration, wrapped
// so every request declares its identity to the resilience round-tripper, which
// emits the span+metric+log. When c.Ping is set the endpoint is then probed once
// at startup (ListBuckets) so that misconfiguration or an unreachable endpoint
// fails fast rather than on first use; the probe is off by default, so an
// endpoint that is not up yet does not block startup.
//
// center is the governance center — the family's sole injection point; the ctor
// reads the resilience and fault authorities from it and bundles them into the
// [cloud.ClientParams] it hands the driver, which
// passes it to [NewClient] — so the client is assembled complete in one step,
// with the zero bundle degrading to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, center *governance.Center) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating s3 client, endpoint=%s region=%s", c.Endpoint, c.Region)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		return nil, err
	}
	// The Driver returned the client complete — identity and governance both
	// applied while it was built. There is no Init hook and nothing else runs
	// after this — the bean is complete when this ctor returns.
	// Fail fast: probe the endpoint once at startup so a misconfigured or
	// unreachable endpoint surfaces during boot rather than on the first
	// request. The probe is [HealthCheck], the single liveness implementation,
	// which goes straight to the raw client — it is a connectivity check, not
	// business traffic. A failure abandons the client, so release what was just
	// applied. Off by default (c.Ping): an endpoint that is not up yet must not
	// block startup.
	if c.Ping {
		if err := HealthCheck(ctx.Context, client); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "s3: startup probe failed: %v", err)
			_ = client.Destroy()
			return nil, errutil.Explain(err, "failed to reach s3 endpoint %s", c.Endpoint)
		}
	}
	log.Infof(ctx.Context, log.TagAppDef, "s3 client initialized, endpoint=%s", c.Endpoint)
	return client, nil
}

// HealthCheck reports whether the S3 endpoint is reachable and the credential
// pair is accepted, by listing buckets. It is a thin readiness probe suitable
// for wiring into a health endpoint. The probe goes straight to the raw client
// on purpose: a readiness check must reflect the backend, not the wrapper.
func HealthCheck(ctx context.Context, client *Client) error {
	_, err := client.Client.ListBuckets(ctx)
	return err
}
