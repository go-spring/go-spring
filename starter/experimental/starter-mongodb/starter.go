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

package StarterMongoDB

import (
	"context"
	"fmt"
	"net"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-mongodb/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func init() {
	// Register multiple MongoDB clients as a group, one per entry under
	// "${spring.mongodb}". A gs.Module (rather than gs.Group) is used so each
	// instance's *mongo.Client bean can be paired with a health.Indicator
	// registered under the same name — and to attach the file:line of this
	// registration to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.mongodb.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.mongodb.instances}", func(name string, c Config) error {

			// The wrapper bean owns the resilience executor + discovery watch, so
			// Init arms it (InitMethod) and Close tears it down (Destroy). The
			// instance's discovery.Discovery backend bean is injected by name from
			// the entry's ${discovery} label (default "default"; optional, so an
			// app with no backend beans at all gets nil here).
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.mongodb.instances."+name+".discovery:=none}?")),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)

			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. The wrapper is what is
			// autowired; the embedded *mongo.Client is handed to the indicator.
			r.Provide(func(w *Client) *health.Indicator {
				return health2.NewClientHealth(name, w.Client)
			}, gs.TagArg(name)).Name("mongo:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new MongoDB client based on the provided configuration,
// wrapped so gs can field-inject resilience + observability and
// Init (InitMethod) can arm them. The command monitor (observability)
// and the dial seam (resilience) are installed dynamically: newClient wires a
// mutable monitor + dialer into the driver, and Init later swaps in
// the observe observer and the resilience-wrapped dial function once the
// injected policy is available. After the client is built it is pinged so that
// misconfiguration or an unreachable server fails fast at startup rather than
// on first use.
//
// When c.ServiceName is set and mesh mode is off, the address is resolved
// through backend (the discovery backend the entry's ${discovery} label
// resolved to): a
// loader-backed dialer is injected as the client's ContextDialer, so each new
// connection dials a currently-live instance picked from the service's endpoint
// snapshot and address changes take effect without rebuilding the client. In
// mesh mode a sidecar owns discovery+LB, so the URI hosts are dialed directly.
// When c.ServiceName is empty this dials the URI hosts directly, unchanged from
// before.
func newClient(ctx *gs.ContextProvider, c Config, backend discovery.Discovery) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating mongodb client, uri=%s service-name=%s", c.URI, c.ServiceName)

	opts := options.Client().ApplyURI(c.URI)
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(c.ServerSelectionTimeout)
	}
	if c.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(c.MaxPoolSize)
	}
	opts.SetMinPoolSize(c.MinPoolSize)
	if c.MaxConnIdleTime > 0 {
		opts.SetMaxConnIdleTime(c.MaxConnIdleTime)
	}
	if c.Username != "" {
		opts.SetAuth(options.Credential{
			Username:      c.Username,
			Password:      c.Password,
			AuthSource:    c.AuthSource,
			AuthMechanism: c.AuthMechanism,
		})
	}
	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mongodb: build TLS failed: %v", err)
		return nil, errutil.Explain(err, "mongodb: build TLS")
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	w := &Client{cfg: c}
	// The command monitor observes operations; it reads the observer lazily so
	// Init can build it from the injected Observability config once
	// the wrapper is field-injected. No commands run before Init.
	opts.SetMonitor(newCommandMonitor(func() *dbObserver { return w.obs.Load() }))

	var baseDial func(ctx context.Context, network, address string) (net.Conn, error)
	pool, err := newPickPool(ctx.Context, c, backend)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mongodb: build discovery resolver failed: %v", err)
		return nil, err
	}
	if pool != nil {
		nd := &net.Dialer{Timeout: c.ConnectTimeout}
		// The discovery dialer ignores the URI address and picks a live
		// endpoint from the loader-backed pool on each new connection.
		baseDial = func(ctx context.Context, network, _ string) (net.Conn, error) {
			ep, err := pool.Pick(loadbalance.PickInfo{})
			if err != nil {
				return nil, err
			}
			return nd.DialContext(ctx, network, ep.Addr)
		}
	}
	if baseDial == nil {
		nd := &net.Dialer{Timeout: c.ConnectTimeout}
		baseDial = nd.DialContext
	}
	// A shared dialer instance is handed to the driver; Init mutates
	// its dial field (wrapping it with resilience.NewDialer) so the swap takes
	// effect without rebuilding the client.
	w.dialer = &dialerWrapper{dial: baseDial}
	opts.SetDialer(w.dialer)

	client, err := mongo.Connect(opts)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mongodb: connect failed: %v", err)
		return nil, fmt.Errorf("mongodb: create client: %w", err)
	}
	w.Client = client

	// Fail fast: verify the server is reachable before handing out the client.
	pingCtx, cancel := pingContext(ctx.Context, c.ConnectTimeout)
	defer cancel()
	if err := client.Ping(pingCtx, nil); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mongodb: ping failed uri=%s: %v", c.URI, err)
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping %s: %w", c.URI, err)
	}
	log.Infof(ctx.Context, log.TagAppDef, "mongodb client initialized, uri=%s", c.URI)
	return w, nil
}

// HealthCheck reports whether the MongoDB client can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client *Client) error {
	return client.Ping(ctx, nil)
}

// pingContext derives a context for the startup ping, bounded by the connect
// timeout when set so the probe cannot hang indefinitely.
func pingContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}
