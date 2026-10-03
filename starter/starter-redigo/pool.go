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

package StarterRedigo

import (
	"context"
	"crypto/tls"
	"go-spring.org/cloud/chain"
	"net"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// Pool is the wrapper bean redigo pools are injected as. It embeds the raw
// *redis.Pool, so the whole raw-pool method set is promoted unchanged. That is
// the right shape here because the pool itself is not instrumented: the command
// chain is composed onto each connection the pool dials ([NewConn], installed
// by the wrapped Dial), so this type is a holder, not a per-command
// interceptor. [NewPool] is the only way to build a Pool, so a pool can never
// exist without its observer and its instrumented Dial wrap. NewPool assembles
// it in ONE phase — observer, resilience executor, endpoint-selection binding,
// and the instrumented Dial wrap are all live on return; there is no separate
// Init.
type Pool struct {
	// *redis.Pool is embedded. The raw pool carries no instrumentation of its
	// own (the wrapped Dial hands out the instrumented Conn), so its methods are
	// promoted rather than re-declared one by one. Pool.Close overrides the
	// promoted Close so teardown also detaches the binding and executor.
	*redis.Pool

	cfg          Config               // address fields feed the governance service label
	exec         chain.Executor       // the executor every command runs under, set by [NewPool] from the governance bundle; always non-nil on a NewPool-built pool
	chain        []CommandInterceptor // user interceptor chain, first entry outermost; nil when none registered
	serviceLabel string               // governance service label (stable per pool)
	lbPool       *loadbalance.Pool    // endpoint-selection pool, nil when discovery is not in effect
	stop         func()               // detaches the endpoint-selection binding
}

// params carries the container's facilities (see [cloud.ClientParams]).
// params.Discovery is the discovery backend the entry's ${discovery} label
// resolved to, already looked up by the starter wiring; a stand-alone NewPool
// caller passes the backend it wants explicitly (nil for a plain Addr dial). It
// rides on the params struct rather than Config so NewPool stays a pure function
// of its inputs — Config is a pure bound value. It is applied HERE, while the
// pool is built, so a Pool cannot exist half-assembled: there is no separate
// apply step and nothing the container has to remember to call. A stand-alone
// caller passes the zero [cloud.ClientParams]; its executor then degrades to
// resilience.Unmanaged — observed, with a one-time warning that no protection
// applies — rather than running bare.
func NewPool(ctx context.Context, c Config, params cloud.ClientParams) (*Pool, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}

	// Bind service discovery; nil resolver means discovery not in effect (no
	// service name / no backend / mesh), so the pool dials the
	// configured Addr directly. Freshness lives inside the backend, so the
	// resolver has no resources to release.
	resolver, err := discovery.NewResolver(ctx, params.Discovery, c.ServiceName,
		discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, err
	}
	// The service label scopes limiter/breaker state AND endpoint selection to
	// this Redis instance (not per command): fall back across the address fields
	// via the shared [resilience.ServiceLabel] helper. Computed here because the
	// pool's selection binding needs it before the executor is built.
	serviceLabel := resilience.ServiceLabel("redigo", c.ServiceName, c.Addr)

	// Endpoint selection rides the shared loadbalance machinery (round-robin
	// here, per opened connection). The tracker makes outlier suspension possible
	// and the binding makes the label's governance rule drive both halves.
	var lb *loadbalance.Pool
	if resolver != nil {
		bal := loadbalance.NewRoundRobin()
		lb = loadbalance.NewPool(resolver, bal)
	}

	pool := newRawPool(c, tlsConfig, lb)
	w := &Pool{Pool: pool, cfg: c, serviceLabel: serviceLabel, lbPool: lb, stop: func() {}}

	// Governance is assembled complete, in one step. The executor the pool's
	// service label resolves to is the stack observe(fault(execFor)): fault wraps
	// the resolved executor's operation fn so injected failures land INSIDE the
	// retry/breaker loop (and so are observed), and the resilience layer emits the
	// span + counter + histogram + access log for the whole call, retries included.
	// A nil Fault is nil-safe (WrapClientExecutor is a transparent pass-through)
	// and the zero bundle degrades to resilience.Unmanaged. Resolution is deferred
	// to call time, so the order relative to the center's wiring is
	// irrelevant; rate / error / latency hot-toggle and the bound protection policy
	// is adopted at runtime through the executor's Refresh.
	w.exec = params.ExecutorFor("redigo", serviceLabel)

	// Endpoint selection rides the shared loadbalance machinery. Binding it here
	// (rather than after assembly) is what makes the pool complete on return, so a
	// custom Driver's pool is governed without any post-construction patching.
	if lb != nil && params.Loadbalance != nil {
		w.stop = params.Loadbalance.Bind(lb, serviceLabel)
	}

	// Install the standard instrumentation: the command observer and the
	// instrumented Dial wrap. The observer is unconditional: without
	// starter-otel the OTel globals are no-ops, so it costs one map lookup per
	// command, and its instruments come from the process-wide set (see
	// [instruments]).
	w.setupDial()
	return w, nil
}

// newRawPool builds the underlying *redis.Pool for NewPool: pool sizing, TLS,
// credentials, and the dial function (static Addr, or discovery-picked endpoints
// when a resolver is in play).
func newRawPool(c Config, tlsConfig *tls.Config, lb *loadbalance.Pool) *redis.Pool {
	return &redis.Pool{
		MaxActive:       c.PoolSize,
		MaxIdle:         c.MaxIdle,
		MaxConnLifetime: c.ConnMaxLifetime,
		Wait:            true,
		Dial: func() (redis.Conn, error) {
			opts := []redis.DialOption{
				redis.DialPassword(c.Password),
				redis.DialConnectTimeout(c.DialTimeout),
				redis.DialReadTimeout(c.ReadTimeout),
				redis.DialWriteTimeout(c.WriteTimeout),
			}
			if c.Username != "" {
				opts = append(opts, redis.DialUsername(c.Username))
			}
			if tlsConfig != nil {
				opts = append(opts,
					redis.DialUseTLS(true),
					redis.DialTLSConfig(tlsConfig),
					redis.DialTLSSkipVerify(c.TLS.InsecureSkipVerify),
				)
			}
			// addr is the static target; with service discovery the resolver
			// overrides it by picking a live endpoint.
			addr := c.Addr
			if lb != nil {
				nd := &net.Dialer{Timeout: c.DialTimeout}
				opts = append(opts, redis.DialContextFunc(
					func(ctx context.Context, network, _ string) (net.Conn, error) {
						ep, err := lb.Pick(loadbalance.PickInfo{})
						if err != nil {
							return nil, err
						}
						conn, derr := nd.DialContext(ctx, network, ep.Addr)
						// The dial outcome is the only signal this picker has;
						// feeding it makes outlier suspension evict an instance
						// that keeps refusing connections.
						lb.Complete(ep, derr)
						return conn, derr
					}))
				// Addr becomes a label for the pool; the dialer picks a live
				// endpoint.
				addr = c.ServiceName
			}
			conn, err := redis.Dial("tcp", addr, opts...)
			if err != nil {
				return nil, err
			}
			if c.DB != 0 {
				_, err = conn.Do("SELECT", c.DB)
				if err != nil {
					conn.Close()
					return nil, err
				}
			}
			return conn, nil
		},
	}
}

// Close tears the pool down: detaches the endpoint-selection binding, closes the
// resilience executor, then the underlying redis pool. Freshness lives inside
// the discovery backend, so there is no per-pool watch to stop. It is the bean
// destroy method; the raw pool's own Close is reached through it, so the wrapper
// overrides rather than delegates it.
func (p *Pool) Close() error {
	p.stop()
	if p.exec != nil {
		_ = p.exec.Close()
	}
	return p.Pool.Close()
}

// UseCommandInterceptor adds per-command interceptors to this pool. Each
// connection the pool dials afterwards runs them outermost (first-added first),
// ahead of the built-in observe span and resilience executor — so a layer can
// short-circuit without starting a span or consuming a breaker permit, rewrite
// the ctx/cmd/args it forwards, or simply observe the outcome. Inject the pool
// in a bean and call this from its Init, before the pool hands out connections;
// already-dialed connections keep the chain they were built with.
func (p *Pool) UseCommandInterceptor(i ...CommandInterceptor) {
	for _, x := range i {
		if x == nil {
			panic("redigo: use nil command interceptor")
		}
	}
	p.chain = append(p.chain, i...)
}

// wrapConn wraps a freshly dialed raw connection in the instrumented Conn.
// Layer order (earlier = outermost): user interceptors first (so a
// short-circuit skips the declared identity and the breaker), then the
// declaration layer, then the resilience executor innermost — the declaration
// reaches the executor, which emits the one span covering the whole call with
// any retries the policy drives. It reads the pool's live state, so
// interceptors added after NewPool still apply to connections dialed later.
func (p *Pool) wrapConn(raw redis.Conn) redis.Conn {
	var layers []CommandInterceptor
	layers = append(layers, p.chain...)
	layers = append(layers, operationInterceptor())
	if p.exec != nil {
		layers = append(layers, resilienceInterceptor(p.exec, p.serviceLabel))
	}
	return NewConn(raw, layers...)
}

// setupDial wraps the pool's Dial / DialContext so every connection handed out
// goes through newConn — the Conn that instruments each command
// through the module-local observe layer (trace span + duration metric +
// access log) and, when governance is applied, through the executor. It is
// called by NewPool after the observer is built; the wrap
// resolves at dial time, so interceptors added afterwards still apply to
// connections dialed later.
//
// The observer rides the OTel globals (starter-otel) and the project log, so
// it needs no per-component adaptation: when starter-otel is absent,
// trace+metric are no-ops and only the access log remains.
func (o *Pool) setupDial() {
	if d := o.Pool.Dial; d != nil {
		o.Pool.Dial = func() (redis.Conn, error) {
			c, err := d()
			if err != nil {
				return nil, err
			}
			return o.wrapConn(c), nil
		}
	}
	if d := o.Pool.DialContext; d != nil {
		o.Pool.DialContext = func(ctx context.Context) (redis.Conn, error) {
			c, err := d(ctx)
			if err != nil {
				return nil, err
			}
			return o.wrapConn(c), nil
		}
	}
}
