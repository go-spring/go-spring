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

	"github.com/gomodule/redigo/redis"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/label"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/connpool"
	"go-spring.org/cloud/governance/dataplane/outbound/loadbalance"
	"go-spring.org/cloud/traffic"
	"go-spring.org/stdlib/errutil"
)

// Pool is the wrapper bean redigo pools are injected as. It holds the
// connection pool ([connpool.Pool], one sub-pool per endpoint) and the command
// chain every command runs under; [Pool.Get] hands out the virtual [Conn]
// whose commands acquire a connection for the peer the chain's addressing
// stage picked. [NewPool] is the only way to build a Pool, so a pool can never
// exist without its executor and its addressing; it assembles everything in
// ONE phase — there is no separate Init.
//
// Pool sizing note: MaxActive is PER ENDPOINT in this model (a pool serving N
// endpoints holds up to N×MaxActive connections), replacing the raw pool's
// process-wide ceiling; ConnMaxLifetime bounds idle reaping.
type Pool struct {
	cfg          Config               // address fields feed the governance service label
	exec         chain.Executor       // the executor every command runs under, set by [NewPool]; always non-nil on a NewPool-built pool
	chain        []CommandInterceptor // user interceptor chain, first entry outermost; nil when none registered
	serviceLabel string               // governance service label (stable per pool)
	prop         traffic.Propagator   // process traffic convention, read when a command's operation is built
	picker       *loadbalance.Chain   // the addressing chain, nil when discovery is not in effect
	staticAddr   string               // the fixed target a nil-picker pool dials
	cpool        *connpool.Pool[redis.Conn]
}

// center carries the container's facilities (see [governance.Runtime]). The
// entry's ${discovery} label rides on the config (c.Discovery, already carrying
// the family default); NewPool resolves it against center.Directory itself.
// It is applied HERE, while the pool is built, so a Pool cannot exist
// half-assembled: there is no separate apply step and nothing the container has
// to remember to call. A stand-alone caller must supply a center holding a
// [resilience.Manager]: [governance.Runtime.ExecutorFor] is required, so a nil
// center is a construction error rather than running bare.
func NewPool(ctx context.Context, c Config, center *governance.Runtime) (*Pool, error) {
	return NewPoolWithObserver(ctx, c, center, nil)
}

// NewPoolWithObserver is the injection point for a custom pool observer. A nil
// observer uses the package default.
func NewPoolWithObserver(ctx context.Context, c Config, center *governance.Runtime, obs connpool.Observer) (*Pool, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}

	// The service label scopes limiter/breaker state AND endpoint selection to
	// this Redis instance (not per command): fall back across the address fields
	// via the shared [label.ServiceLabel] helper. Computed here because the
	// chain's registration needs it before the executor is built.
	serviceLabel := label.ServiceLabel("redigo", c.ServiceName, c.Addr)

	// Addressing rides the chain: with discovery in effect a loadbalance chain
	// is registered under the service label and its Picker is injected into the
	// executor, so every ROUND of a call picks its endpoint — a retry re-picks,
	// excluding the peers the call already failed against — and settles the
	// outcome with the picker's tracker. Mesh mode wins over a configured
	// service-name: the sidecar owns discovery+LB, so no picker is built and
	// every round runs against the configured Addr.
	var picker *loadbalance.Chain
	staticAddr := c.Addr
	if center.UsesDiscovery(c.ServiceName) {
		backend, _ := center.Directory.Get(c.Discovery)
		if backend == nil {
			return nil, errutil.Explain(nil, "redis: instance routes by service-name but cites discovery backend %q, which names no bean", c.Discovery)
		}
		resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName, c.Scheme)
		if err != nil {
			return nil, err
		}
		lb, err := center.LoadBalance.Chain(ctx, resolver, serviceLabel)
		if err != nil {
			return nil, err
		}
		picker = lb
		staticAddr = c.ServiceName // Addr becomes a label for the chain; the picker picks live endpoints
	}

	cpool, err := connpool.NewPool(connpool.PoolConfig[redis.Conn]{
		Dial:           dialRaw(c, tlsConfig),
		CloseConn:      func(rc redis.Conn) error { return rc.Close() },
		MaxActive:      c.PoolSize,
		MaxIdle:        c.MaxIdle,
		IdleTimeout:    c.ConnMaxLifetime,
		DialTimeout:    c.DialTimeout,
		AcquireTimeout: c.DialTimeout,
		Observer:       obs,
	})
	if err != nil {
		return nil, err
	}

	w := &Pool{
		cfg:          c,
		serviceLabel: serviceLabel,
		prop:         center.Traffic,
		picker:       picker,
		staticAddr:   staticAddr,
		cpool:        cpool,
	}

	// Governance is assembled complete, in one step. The executor the pool's
	// service label resolves to is the stack observe(resilience(addressing(
	// fault))): addressing picks the peer per round, fault wraps the operation
	// fn so injected failures land INSIDE the retry/breaker loop (and so are
	// observed), and the resilience layer emits the span + counter + histogram
	// + access log for the whole call, retries included. The center is
	// required to carry a [resilience.Manager]; governance.Runtime.ExecutorFor
	// panics without one.
	w.exec = center.ExecutorFor("redigo", serviceLabel, picker)
	return w, nil
}

// dialRaw builds the pool's dial function: TLS, credentials, timeouts and the
// SELECT of the configured logical DB. It dials exactly the address it is
// handed — the chain's addressing stage chose it — and its failure returns
// marked as a connect error by the connpool, where the chain's settlement
// reads it.
func dialRaw(c Config, tlsConfig *tls.Config) func(ctx context.Context, addr string) (redis.Conn, error) {
	return func(ctx context.Context, addr string) (redis.Conn, error) {
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
		conn, err := redis.DialContext(ctx, "tcp", addr, opts...)
		if err != nil {
			return nil, err
		}
		if c.DB != 0 {
			if _, err = conn.Do("SELECT", c.DB); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
		return conn, nil
	}
}

// Get returns the virtual [Conn] whose every command runs the full chain —
// user interceptors, then the resilience executor with addressing per round —
// acquiring a pooled connection for the peer each round picked. The Conn holds
// no connection of its own between commands; its Close returns nothing because
// nothing is held.
func (p *Pool) Get() *Conn {
	var layers []CommandInterceptor
	layers = append(layers, p.chain...)
	if p.exec != nil {
		layers = append(layers, resilienceInterceptor(p.exec, p.serviceLabel, p.prop))
	}
	return NewConn(commandSink(p), layers...)
}

// commandSink is the terminal command handler: acquire a connection for the
// peer the round addressed, run the command, and return the connection —
// parked when it served cleanly, discarded when the command failed in a way
// that leaves it suspect.
func commandSink(p *Pool) CommandHandler {
	return func(ctx context.Context, peer chain.Peer, cmd string, args []interface{}) (interface{}, error) {
		addr := peer.Addr
		if addr == "" {
			addr = p.staticAddr
		}
		ref, err := p.cpool.Acquire(ctx, addr)
		if err != nil {
			return nil, err
		}
		reply, err := doCommand(ref.Raw(), ctx, cmd, args)
		if err != nil {
			ref.Close()
		} else {
			ref.Release()
		}
		return reply, err
	}
}

// doCommand runs one command on one raw connection, preferring the
// context-aware path so the round's deadline reaches the wire.
func doCommand(raw redis.Conn, ctx context.Context, cmd string, args []interface{}) (interface{}, error) {
	if ctx == nil || ctx == context.Background() {
		return raw.Do(cmd, args...)
	}
	type ctxDoer interface {
		DoContext(context.Context, string, ...interface{}) (interface{}, error)
	}
	if c, ok := raw.(ctxDoer); ok {
		return c.DoContext(ctx, cmd, args...)
	}
	return raw.Do(cmd, args...)
}

// Stats reports the pool's live accounting (held, parked and endpoint count).
func (p *Pool) Stats() connpool.PoolStats { return p.cpool.Stats() }

// Close tears the pool down: the connection pool, then the resilience
// executor. Freshness lives inside the discovery backend, so there is no
// per-pool watch to stop. It is the bean destroy method.
func (p *Pool) Close() error {
	var first error
	if err := p.cpool.Close(); err != nil && first == nil {
		first = err
	}
	if p.exec != nil {
		if err := p.exec.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// UseCommandInterceptor adds per-command interceptors to this pool. Every
// command run afterwards goes through them outermost (first-added first),
// ahead of the built-in observe span and resilience executor — so a layer can
// short-circuit without starting a span or consuming a breaker permit, rewrite
// the ctx/cmd/args it forwards, or simply observe the outcome. Inject the pool
// in a bean and call this from its Init; it reads the pool's live state, so
// interceptors added after NewPool apply to commands issued later.
func (p *Pool) UseCommandInterceptor(i ...CommandInterceptor) {
	for _, x := range i {
		if x == nil {
			panic("redigo: use nil command interceptor")
		}
	}
	p.chain = append(p.chain, i...)
}
