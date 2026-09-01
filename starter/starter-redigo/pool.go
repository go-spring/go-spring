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
	observe "go-spring.org/cloud/observe"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Pool is the wrapper bean redigo pools are injected as. It embeds
// the concrete *redis.Pool (so Get/Stats/etc. promote unchanged). NewPool
// assembles it in ONE phase — observer, resilience executor, and the
// instrumented Dial wrap are all live on return; there is no separate Init.
type Pool struct {
	*redis.Pool

	cfg      Config               // address fields feed the resilience resource label
	obs      *observe.Observer    // nil when observe disabled (near-zero-cost wrapper)
	exec     resilience.Executor  // resolved via resilience.ExecutorFor; no-op when governance is off
	chain    []CommandInterceptor // user interceptor chain, first entry outermost; nil when none registered
	resource string               // resilience resource label (stable per pool)
	resolver *discovery.Resolver  // live when ServiceName drives discovery; Close stops its watch
}

func NewPool(ctx context.Context, c Config, obs observe.ObserveConfig) (*Pool, error) {
	tlsConfig, err := c.TLS.Build()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}

	resolver, err := discovery.NewResolver(ctx, c.Discovery, c.ServiceName,
		discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, err
	}

	pool := newRawPool(c, tlsConfig, resolver)
	w := &Pool{Pool: pool, cfg: c, resolver: resolver}

	// Arm the standard instrumentation: the command observer (when enabled),
	// the resilience executor, and the instrumented Dial wrap.
	if c.ObserveEnabled {
		w.obs = observe.NewDB("redis", obs)
	}
	if err := w.setupResilience(obs); err != nil {
		_ = w.Close()
		return nil, err
	}

	w.setupDial()
	return w, nil
}

// newRawPool builds the underlying *redis.Pool for NewPool: pool sizing, TLS,
// credentials, and the dial function (static Addr, or discovery-picked endpoints
// when a resolver is in play).
func newRawPool(c Config, tlsConfig *tls.Config, resolver *discovery.Resolver) *redis.Pool {
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
			if resolver != nil {
				nd := &net.Dialer{Timeout: c.DialTimeout}
				opts = append(opts, redis.DialContextFunc(
					func(ctx context.Context, network, _ string) (net.Conn, error) {
						ep, err := resolver.Pick()
						if err != nil {
							return nil, err
						}
						return nd.DialContext(ctx, network, ep.Addr)
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

// Close tears the pool down: closes the resilience executor (if armed), stops
// the discovery-resolver watch (when discovery is in use), then closes the
// underlying redis pool. It shadows the embedded (*redis.Pool).Close so a
// plain Close cannot leak the resolver watch.
func (p *Pool) Close() error {
	if p.exec != nil {
		_ = p.exec.Close()
	}
	if p.resolver != nil {
		_ = p.resolver.Stop()
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

// setupResilience builds the executor stack from the governance-driven
// resilience seam and the process-wide fault injector, storing it on the pool
// so every Conn threads each command through it. Both come from neutral seams
// ([resilience.ExecutorFor] / [fault.InjectorFor]) that starter-govern backs
// with the governance center, so this pool neither injects nor names
// cloud/governance.
//
// The stack is observe( fault( execFor ) ): fault wraps the resolved executor's
// operation fn so injected failures land INSIDE the retry/breaker loop (and so
// are observed), and observe sits outermost so trips / rejects / retries emit
// span + counter + histogram + access log (the resilience core emits none).
// fault is nil-safe: when no injector is registered (governance off / fault
// disabled) WrapExecutor returns the inner executor unchanged, so the fault
// layer is a transparent pass-through. Rate/Error/Latency/Enabled hot-toggle
// at runtime via the center's single OnChanged without a restart.
//
// On a config change the bound policy is adopted without a restart via the
// executor's Refresh seam. obs feeds the resilience observer wrap; it is the
// same global policy the command observer uses.
func (o *Pool) setupResilience(obs observe.ObserveConfig) error {
	// Scope limiter/breaker state per Redis instance (not per command): fall
	// back across the address fields via the shared [resilience.ResourceLabel]
	// helper.
	o.resource = resilience.ResourceLabel("redigo", o.cfg.ServiceName, o.cfg.Addr)

	// The resilience executor is resolved through the NEUTRAL provider seam
	// [resilience.ExecutorFor]: starter-govern registers a provider backed by
	// the governance center, so this pool gets its timeout/retry/breaker policy
	// WITHOUT injecting or naming cloud/governance. When governance is not
	// configured the seam yields a transparent no-op executor, so this call is
	// always safe. Resolution is deferred to call time, hence the order of this
	// setup relative to starter-govern's wiring is irrelevant.
	exec := fault.WrapExecutor(resilience.ExecutorFor(o.resource))

	o.exec = resilience.WrapExecutor(exec, "redigo", obs)
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
	if p.obs != nil {
		layers = append(layers, observeInterceptor(p.obs))
	}
	if p.exec != nil {
		layers = append(layers, resilienceInterceptor(p.exec, p.resource))
	}
	return NewConn(raw, layers...)
}

// setupDial wraps the pool's Dial / DialContext so every connection handed out
// goes through newConn — the Conn that instruments each command
// through the shared observe kit (trace span + duration/in-flight metric +
// access log) and, when resilience is armed, through the executor. It is
// called by NewPool after the observer + executor are built; the wrap
// resolves at dial time, so interceptors added afterwards still apply to
// connections dialed later.
//
// The kit rides the OTel globals (starter-otel) and the project log, so this is
// a near-zero-cost opt-in that needs no per-component adaptation: when
// starter-otel is absent, trace+metric are no-ops; the access log is gated by
// cfg.Level (default brief).
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
