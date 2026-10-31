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

package StarterNeo4j

import (
	"context"
	"time"

	"go-spring.org/cloud/governance"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Neo4j clients as a group, one per entry under
	// "${spring.neo4j}". A gs.Module (rather than gs.Group) is used so each
	// instance's neo4j.DriverWithContext bean can be paired with a health.Indicator
	// registered under the same name — and to attach the file:line of this
	// registration to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.neo4j.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.neo4j.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.neo4j.instances", "spring.neo4j.default")
		return conf.BindEach(p, "${spring.neo4j.instances}", func(name string, c Config) error {
			// newClient reads the authorities off the injected runtime
			// and hands the center straight to the driver, which passes it to
			// [NewClient] — so the client is assembled complete in one step. The
			// wrapper bean owns the resulting resilience executor and Destroy
			// tears it down. The entry's ${discovery} label is bound as a value
			// (family default "none", meaning "no backend") and folded into the
			// config; the driver resolves it against the center's discovery
			// directory. The Driver bean
			// is selected by the entry's ${driver} key: unset → "?" (nullable
			// by-type — injects the single Driver bean when one is provided,
			// nil otherwise, and the ctor falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that
			// does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.neo4j.instances."+name+".discovery:=${spring.neo4j.default.discovery:=none}}")),
				gs.IndexArg(3, gs.TagArg("${spring.neo4j.instances."+name+".driver:=${spring.neo4j.default.driver:=?}}")),
				// The runtime is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// wrapper just registered above by name. Its probe only calls
			// HealthCheck (see health.go). Skipped when c.Health is false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("neo4j:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new Neo4j client based on the provided configuration.
// When c.Ping is set, connectivity is verified after the driver is built so
// that misconfiguration or an unreachable server fails fast at startup rather
// than on first query; the probe is off by default, so a server that is not up
// yet does not block startup.
//
// Observability note: the neo4j-go-driver speaks the binary Bolt protocol and
// ships no official OpenTelemetry instrumentation, nor a command-monitor hook
// comparable to the SQL/MongoDB drivers, so there is no transparent seam that
// sees every request. The starter therefore exposes an opt-in call-site seam
// ([Query] / [Operation]) that builds each operation's semantic identity and
// passes it explicitly to the executor; the
// resilience layer — installed by [NewClient] — is what emits the
// span, the db.client.* metrics and the access log from that identity. Calls
// that bypass the seam (a bare neo4j.ExecuteQuery / session.Run) are unobserved
// and unguarded, a documented gap driven by upstream driver support, not an
// oversight.
//
// When in-process discovery applies (a service name is set and the process is not in mesh mode), the driver builds a Resolver
// against the backend the entry's ${discovery} label names, looked up in the
// center's discovery directory, picks one endpoint, and splices its address into
// the URI host. That pick is a seed: the driver exposes no dialer injection
// point, so a running client does not re-pick per query. What does keep
// following the naming service is the driver's AddressResolver hook, installed
// by CreateClient over the same resolver — it is how a neo4j:// (routing) client
// re-finds the cluster after the seeded host disappears. In mesh mode the
// sidecar owns discovery+LB, so the URI is used unchanged. See Config.ServiceName.
//
// discoveryLabel is the entry's ${discovery} label (already carrying the family
// default). It is folded into the config so the driver resolves the backend
// itself — see the fold below.
//
// center is the runtime — the family's sole injection point. The ctor
// hands the center straight to the driver, which passes it to [NewClient] — so
// the client is assembled complete in one step, with the resilience manager
// required: the container supplies it via the runtime, so a center
// without one fails at construction.
func newClient(ctx *gs.ContextProvider, c Config, discoveryLabel string, d Driver, center *governance.Runtime) (*Client, error) {
	// The client's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context,
		log.String("uri", c.URI),
		log.String("service_name", c.ServiceName))

	log.Debug(cctx, log.TagAppDef, "creating neo4j client", nil)

	// The wiring tag applies the family-default discovery fallback (discoveryLabel);
	// fold it into the config so the driver reads one label — c.Discovery — and
	// resolves the backend (and seeds the URI) itself, failing construction if it
	// names no bean. "none" is the tag's sentinel for "no backend" and is left as
	// an empty c.Discovery.
	if discoveryLabel != "" && discoveryLabel != "none" {
		c.Discovery = discoveryLabel
	}

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c, center)
	if err != nil {
		log.Error(cctx, log.TagAppDef, err, "neo4j: create client failed")
		return nil, errutil.Explain(err, "failed to create neo4j client")
	}
	// The Driver returned the client complete — identity and governance both
	// applied while it was built. There is no Init hook and nothing else runs
	// after this — the bean is complete when this ctor returns.
	// Fail fast: verify the server is reachable before handing out the driver.
	// HealthCheck goes straight to the raw driver on purpose: it is a
	// connectivity check, not business traffic, so it must not open a span or
	// spend limiter/breaker budget. A failure abandons the client, so release
	// what was just applied. Off by default (c.Ping): a server that is not up
	// yet must not block startup.
	if c.Ping {
		vctx, cancel := verifyContext(ctx.Context, c.SocketConnectTimeout)
		defer cancel()
		if err := HealthCheck(vctx, client); err != nil {
			log.Error(cctx, log.TagAppDef, err, "neo4j: verify connectivity failed")
			_ = client.Destroy()
			return nil, errutil.Explain(err, "failed to verify neo4j connectivity: %s", c.URI)
		}
	}
	log.Info(cctx, log.TagAppDef, "create neo4j client success")
	return client, nil
}

// verifyContext derives a context for the startup connectivity check, bounded by
// the socket connect timeout when set so the probe cannot hang indefinitely.
func verifyContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}
