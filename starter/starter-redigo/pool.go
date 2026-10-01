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
	"net"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Pool is the wrapper bean redigo pools are injected as. It embeds
// the concrete *redis.Pool (so Get/Stats/etc. promote unchanged). NewPool
// assembles it in ONE phase — observer, resilience executor, and the
// instrumented Dial wrap are all live on return; there is no separate Init.
type Pool struct {
	*redis.Pool

	cfg     Config                    // address fields feed the governance service label
	exec    resilience.ClientExecutor // armed by ArmGovernance; observe-only when governance is off
	chain   []CommandInterceptor      // user interceptor chain, first entry outermost; nil when none registered
	service string                    // governance service label (stable per pool)
	lbPool  *loadbalance.Pool         // endpoint-selection pool, nil when discovery is not in effect
	stop    func()                    // detaches the endpoint-selection binding
}

// backend is the discovery backend the entry's ${discovery} label resolved to,
// already looked up by the starter wiring; a stand-alone NewPool caller passes
// the backend it wants explicitly (nil for a plain Addr dial). It is passed as
// an argument rather than carried on Config so NewPool stays a pure function of
// its inputs — Config is a pure bound value.
func NewPool(ctx context.Context, c Config, backend discovery.Discovery) (*Pool, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}

	// Bind service discovery; nil resolver means discovery not in effect (no
	// service name / no backend / mesh), so the pool dials the
	// configured Addr directly. Freshness lives inside the backend, so the
	// resolver has no resources to release.
	resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName,
		discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, err
	}
	// The service label scopes limiter/breaker state AND endpoint selection to
	// this Redis instance (not per command): fall back across the address fields
	// via the shared [resilience.ServiceLabel] helper. Computed here because the
	// pool's selection binding needs it before the executor is built.
	service := resilience.ServiceLabel("redigo", c.ServiceName, c.Addr)

	// Endpoint selection rides the shared loadbalance machinery (round-robin
	// here, per opened connection). The tracker makes outlier suspension possible
	// and the binding makes the label's governance rule drive both halves.
	var lb *loadbalance.Pool
	if resolver != nil {
		bal := loadbalance.NewRoundRobin()
		lb = loadbalance.NewPool(resolver, bal)
	}

	pool := newRawPool(c, tlsConfig, lb)
	w := &Pool{Pool: pool, cfg: c, service: service, lbPool: lb, stop: func() {}}

	// Arm the standard instrumentation: the command observer and the
	// instrumented Dial wrap. The observer is unconditional: without
	// starter-otel the OTel globals are no-ops, so it costs one map lookup per
	// command, and its instruments come from the process-wide set (see
	// [instruments]). Governance (the resilience executor and the
	// endpoint-selection binding) is armed separately by [Pool.ArmGovernance],
	// which the gs wiring calls with the injected beans — see that method for why
	// it is not part of this pure assembly.
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
// resilience executor (if armed), then the underlying redis pool. Freshness
// lives inside the discovery backend, so there is no per-pool watch to stop.
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

// ArmGovernance attaches the governance-driven stack to the pool, so every Conn
// threads each command through it: the resilience executor this pool's service
// label resolves to, and the process-wide fault injector wrapping it. The three
// authorities are the beans starter-governance registers, handed in by the gs
// wiring; a standalone caller passes nil, which this method normalizes to a
// fresh unarmed authority — exactly "governance off", so callers need no nil
// branches of their own.
//
// The stack is observe( fault( execFor ) ): fault wraps the resolved executor's
// operation fn so injected failures land INSIDE the retry/breaker loop (and so
// are observed), and observe sits outermost so trips / rejects / retries emit
// span + counter + histogram + access log (the resilience core emits none).
// inj is nil-safe: with no injector, WrapClientExecutor returns the inner executor
// unchanged, so the fault layer is a transparent pass-through. Rate / Error /
// Latency / Enabled hot-toggle at runtime because the center swaps the
// injector's config in place; the bound protection policy is adopted the same
// way, through the executor's Refresh, and a selection change reaches the pool
// through its binding. None of it needs a restart.
func (o *Pool) ArmGovernance(mgr *resilience.Manager, inj *fault.Injector, lbMgr *loadbalance.Manager) error {

	// Resolution is deferred to call time, so the order of this arming relative
	// to starter-governance's wiring is irrelevant.
	o.exec = fault.WrapClientExecutor(mgr.ClientExecutorFor("redigo", o.service), o.service, inj)

	// Endpoint selection rides the shared loadbalance machinery. Binding it here
	// (rather than during assembly) is what keeps the pool's own construction a
	// pure function of its Config, and lets a custom Driver's pool be governed
	// without changing the Driver interface.
	if o.lbPool != nil {
		o.stop = lbMgr.Bind(o.lbPool, o.service)
	}
	return nil
}

// wrapConn wraps a freshly dialed raw connection in the instrumented Conn.
// Layer order (earlier = outermost): user interceptors first (so a
// short-circuit skips the span and the breaker), then the observe span, then
// the resilience executor innermost — the span wraps the executor, so one
// Execute with any retries the policy drives shares a single span. It reads
// the pool's live state, so interceptors added after NewPool still apply to
// connections dialed later.
func (p *Pool) wrapConn(raw redis.Conn) redis.Conn {
	var layers []CommandInterceptor
	layers = append(layers, p.chain...)
	layers = append(layers, observeInterceptor())
	if p.exec != nil {
		layers = append(layers, resilienceInterceptor(p.exec, p.service))
	}
	return NewConn(raw, layers...)
}

// setupDial wraps the pool's Dial / DialContext so every connection handed out
// goes through newConn — the Conn that instruments each command
// through the module-local observe layer (trace span + duration metric +
// access log) and, when resilience is armed, through the executor. It is
// called by NewPool after the observer + executor are built; the wrap
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

// startupPing dials one bare connection and PINGs it so a misconfigured
// address or unreachable server surfaces during boot rather than on the first
// request.
//
// It uses pool.Dial (a non-pooled dial) instead of pool.Get: a conn borrowed
// via Get is returned to the idle pool on Close, and that happens *before*
// NewPool wraps pool.Dial with the Conn — so the stale raw conn would later be
// handed out with no instrumentation and silently bypass resilience. Dialing
// directly keeps it out of the pool. Only runs when Config.StartupPing is set.
func startupPing(ctx context.Context, pool *redis.Pool) error {
	conn, err := pool.Dial()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "redigo: startup ping failed: %v", err)
		return errutil.Explain(err, "redis: startup ping failed")
	}
	_, pingErr := conn.Do("PING")
	_ = conn.Close()
	if pingErr != nil {
		log.Errorf(ctx, log.TagAppDef, "redigo: startup ping failed: %v", pingErr)
		return errutil.Explain(pingErr, "redis: startup ping failed")
	}
	return nil
}
