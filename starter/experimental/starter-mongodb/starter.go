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
	"net"
	"sync/atomic"
	"time"

	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func init() {
	// Register multiple MongoDB clients as a group, one per entry under
	// "${spring.mongodb}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration to
	// the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.mongodb.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.mongodb.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.mongodb.instances", "spring.mongodb.default")
		return conf.BindEach(p, "${spring.mongodb.instances}", func(name string, c Config) error {

			// The wrapper bean owns the resilience executor + discovery watch:
			// newClient builds them and Destroy tears them down, so there is no
			// Init hook. The instance's discovery.Discovery backend bean is
			// injected by name from the entry's ${discovery} label (default
			// "default"; optional, so an app with no backend beans at all gets
			// nil here). The trailing governance beans (*resilience.Manager /
			// *fault.Injector / *loadbalance.Manager) are injected as beans (see
			// the per-arg note below).
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.mongodb.instances."+name+".discovery:=${spring.mongodb.default.discovery:=none}}?")),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(3, gs.TagArg("")),
				gs.IndexArg(4, gs.TagArg("")),
				gs.IndexArg(5, gs.TagArg("")),
			).Name(name).Destroy((*Client).Destroy).Caller(1)

			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. The probe goes straight to
			// the wrapper's raw client. Skipped when c.Health is false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("mongo:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new MongoDB client based on the provided configuration,
// wrapped so every command is observed and every dial is governed. It is the
// client's builder: the command monitor (observability), the shared dialer and
// the governance bundle are all assembled here — the monitor reads the observer
// lazily through a holder, [NewClient] resolves the executor off the bundle, and
// this ctor then wraps the shared dial function with that executor (its
// protection rides the dial layer). When c.Ping is set the client is then
// pinged so that misconfiguration or an unreachable server fails fast at
// startup rather than on first use; the probe is off by default so a server
// that is not up yet does not block startup. There is no Init hook: the client
// is complete when this ctor returns.
//
// When c.ServiceName is set and mesh mode is off, the address is resolved
// through backend (the discovery backend the entry's ${discovery} label
// resolved to): a loader-backed dialer is injected as the client's
// ContextDialer, so each new connection dials a currently-live instance picked
// from the service's endpoint snapshot and address changes take effect without
// rebuilding the client. In mesh mode a sidecar owns discovery+LB, so the URI
// hosts are dialed directly. When c.ServiceName is empty this dials the URI
// hosts directly, unchanged from before.
//
// mgr, inj and lbMgr are the governance beans the container injects (all nil in
// a standalone, non-gs call). They are bundled into one [cloud.ClientParams]
// handed to [NewClient], which resolves the resilience executor from it;
// lbMgr (as the bundle's Loadbalance) additionally binds the discovery pick
// pool this constructor builds, which is where the pool first exists.
func newClient(ctx *gs.ContextProvider, c Config, backend discovery.Discovery,
	mgr *resilience.Manager, inj *fault.Injector, lbMgr *loadbalance.Manager) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating mongodb client, uri=%s service-name=%s", c.URI, c.ServiceName)

	// The container's governance capabilities, bundled so the client is
	// assembled complete in one step (see [cloud.ClientParams]). All three may
	// be nil in a standalone, non-gs call; the bundle then degrades to an
	// observed-only, loudly-unmanaged executor.
	params := cloud.ClientParams{Resilience: mgr, Fault: inj, Loadbalance: lbMgr, Discovery: backend}

	opts := options.Client().ApplyURI(c.URI)
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(c.ServerSelectionTimeout)
	}
	// The pool is sized once, here, so the governance rule's MaxConns — the
	// resource half of isolation, next to the bulkhead's concurrency half — is
	// read at construction too, and wins over the per-instance key so one place
	// configures both halves.
	label := resilience.ServiceLabel("mongodb", c.ServiceName, c.URI)
	if n := params.PolicyFor(label).MaxConns; n > 0 {
		opts.SetMaxPoolSize(uint64(n))
	} else if c.MaxPoolSize > 0 {
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

	// The command monitor observes operations; it reads the observer lazily so
	// it is safe to install before the wrapper (and its observer) exists — the
	// monitor is part of the options the driver is built with, so it must be set
	// before Connect, while NewClient builds the observer only after Connect.
	// The holder bridges the gap; no commands are observed until it is filled.
	var obsRef atomic.Pointer[dbObserver]
	opts.SetMonitor(newCommandMonitor(func() *dbObserver { return obsRef.Load() }))

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
			conn, derr := nd.DialContext(ctx, network, ep.Addr)
			// The dial outcome is the only signal this picker has; feeding it
			// makes outlier suspension evict an instance that keeps refusing
			// connections.
			pool.Complete(ep, derr)
			return conn, derr
		}
	}
	if baseDial == nil {
		nd := &net.Dialer{Timeout: c.ConnectTimeout}
		baseDial = nd.DialContext
	}
	// A shared dialer instance is handed to the driver. Its dial field is
	// swapped for the resilience-wrapped one below (the swap takes effect
	// without rebuilding the client), so governance rides the dial layer.
	dialer := &dialerWrapper{dial: baseDial}
	opts.SetDialer(dialer)

	raw, err := mongo.Connect(opts)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mongodb: connect failed: %v", err)
		return nil, errutil.Explain(err, "mongodb: create client")
	}
	// NewClient resolves the executor off the governance bundle and stores the
	// service label; the client's identity, observe and governance all come from
	// this one call.
	w := NewClient(raw, c, params)
	obsRef.Store(w.obs)
	if pool != nil {
		// Bind the pool to the service's governance label so one
		// spring.governance.client.rules[N] rule drives both its balancing
		// strategy and its protection executor. A nil manager is the standalone
		// case (no container); an unwrapped one is exactly "governance off",
		// where Bind is a no-op — so normalizing here keeps the binding free of
		// a nil branch.
		w.stop = params.Loadbalance.Bind(pool, serviceLabel(c))
	}
	// This starter's protection rides the dial layer (there is no per-command
	// hook), so the executor is consumed here rather than by a per-call seam:
	// wrap the shared dialer's dial function with it and swap it in. Done before
	// the probe, so the client is fully assembled when it is checked.
	dialer.dial = resilience.NewDialer(dialer.dial, w.exec)

	// Fail fast: verify the server is reachable before handing out the client.
	// The probe is [HealthCheck], the single liveness implementation (a
	// connectivity check going straight to the raw client); on failure the
	// client just assembled is released, executor and connection together.
	// Off by default (c.Ping): a server that is not up yet must not block
	// startup, so this runs only when explicitly requested.
	if c.Ping {
		pingCtx, cancel := pingContext(ctx.Context, c.ConnectTimeout)
		defer cancel()
		if err := HealthCheck(pingCtx, w); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "mongodb: ping failed uri=%s: %v", c.URI, err)
			_ = w.Destroy()
			return nil, errutil.Explain(err, "mongodb: ping %s", c.URI)
		}
	}
	log.Infof(ctx.Context, log.TagAppDef, "mongodb client initialized, uri=%s", c.URI)
	return w, nil
}

// HealthCheck reports whether the MongoDB client can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint. The probe
// goes straight to the raw client on purpose: a readiness check must reflect
// the backend, not the rate limiter.
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
