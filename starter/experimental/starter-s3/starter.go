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

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-s3/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple S3 clients as a group, one per entry under
	// "${spring.s3}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.s3"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.s3}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so Init arms it
			// (InitMethod) and Destroy tears it down.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("?")),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) *health.Indicator {
				return health2.NewClientHealth(name, w.Client)
			}, gs.TagArg(name)).Name("s3:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new S3 client based on the provided configuration. The
// endpoint is probed once at startup (ListBuckets) so that misconfiguration or
// an unreachable endpoint fails fast rather than on first use.
func newClient(ctx *gs.ContextProvider, c Config, d Driver) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating s3 client, endpoint=%s region=%s", c.Endpoint, c.Region)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	w := &Client{Client: cl, cfg: c}
	// The DefaultDriver attaches a dynamic transport (its executor swapped in
	// by Init); pick it up so the wrapper can arm it. Custom drivers may not
	// install one — resilience is then simply unavailable for that client.
	if v, ok := dynamicTransports.LoadAndDelete(cl); ok {
		w.dyn = v.(*dynamicTransport)
	}
	if err := HealthCheck(ctx.Context, w); err != nil {
		return nil, errutil.Explain(err, "failed to reach s3 endpoint %s", c.Endpoint)
	}
	return w, nil
}

// HealthCheck reports whether the S3 endpoint is reachable and the credential
// pair is accepted, by listing buckets. It is a thin readiness probe suitable
// for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client *Client) error {
	_, err := client.ListBuckets(ctx)
	return err
}
