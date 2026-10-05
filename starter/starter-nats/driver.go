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

// driver.go is the "construction seam" concept: the Driver interface + the
// bundled DefaultDriver, which owns the raw connection assembly (URL/options/auth/
// TLS + nats.Connect) and wraps the result into the exported *Conn (identity +
// JetStream context). It mirrors starter-memcached's driver.go.
package StarterNats

import (
	"context"
	"go-spring.org/cloud/governance"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/cloud"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a NATS connection. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterNats.Driver); when none is present,
// starter-nats falls back to the bundled [DefaultDriver] inside connection
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Conn] — the wrapper apps inject —
// not the raw *nats.Conn, so a driver takes part in the type the rest of the
// ecosystem sees and future wrapper capabilities are reachable from it. It
// returns the connection COMPLETE: the identity, the JetStream context (when
// enabled) and the resilience executor are all applied while it is built — see
// [NewConn], which fixes the identity and the governance exec — and nothing
// patches the connection afterwards.
//
// params supplies the container's facilities (see [cloud.ClientParams]); it is
// one struct rather than a parameter per capability so this interface — which
// every company driver implements — stays stable as capabilities are added. A
// driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every connection under
// ${spring.nats} is built through it, and per-instance differences are expressed
// through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Conn, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient dials NATS from the provided configuration and wraps the
// connection into a [Conn]. It owns the raw connection assembly — the
// name/reconnect/timeout options, the async error/disconnect/reconnect/close
// handlers (which both log and count, see connStateCounter), the user/token/
// creds/nkey auth and TLS — and then hands the raw connection to [NewConn], which
// fixes its identity and applies the params bundle.
//
// A custom Driver that omits the handlers therefore reports no connection-layer
// events at all, rather than losing them to a starter-side override: nats.Conn's
// Set*Handler methods replace a slot, so the two cannot be layered.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Conn, error) {
	// Connection-layer events are counted as well as logged, from this one
	// place (see connStateCounter for why the pair cannot be split).
	connState := newConnStateCounter()
	opts := []nats.Option{
		nats.Name(c.Name),
		nats.MaxReconnects(c.MaxReconnects),
		nats.ReconnectWait(c.ReconnectWait),
		nats.Timeout(c.ConnectTimeout),
		nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
			subj := ""
			if sub != nil {
				subj = sub.Subject
			}
			log.Error(ctx, log.TagAppDef, err, log.String("subject", subj), log.Msg("nats async error"))
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Warn(ctx, log.TagAppDef, append(connState.record(ctx, connDisconnected),
				log.Err(err),
				log.Msg("nats disconnected"))...)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Info(ctx, log.TagAppDef, append(connState.record(ctx, connReconnected),
				log.String("url", nc.ConnectedUrl()),
				log.Msg("nats reconnected"))...)
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Info(ctx, log.TagAppDef, append(connState.record(ctx, connClosed),
				log.Msg("nats connection closed"))...)
		}),
	}
	if c.Username != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	if c.Token != "" {
		opts = append(opts, nats.Token(c.Token))
	}
	if c.CredsFile != "" {
		opts = append(opts, nats.UserCredentials(c.CredsFile))
	}
	if c.NKeyFile != "" {
		opt, err := nats.NkeyOptionFromSeed(c.NKeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "failed to load nats nkey seed: %s", c.NKeyFile)
		}
		opts = append(opts, opt)
	}
	if c.TLS.Enabled {
		tlsCfg, err := c.TLS.BuildClient()
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, err, "nats build TLS failed")
			return nil, errutil.Explain(err, "nats: build TLS")
		}
		if tlsCfg != nil {
			opts = append(opts, nats.Secure(tlsCfg))
		} else {
			opts = append(opts, nats.Secure())
		}
	}

	nc, err := nats.Connect(c.URL, opts...)
	if err != nil {
		log.Error(ctx, log.TagAppDef, err, log.String("url", c.URL), log.Msg("nats connect failed"))
		return nil, errutil.Explain(err, "failed to connect nats: %s", c.URL)
	}
	// NewConn is the only way to build a Conn: identity and the governance
	// executor are both applied here, so the driver returns a connection that is
	// complete (see [Driver]).
	conn := NewConn(nc, c.URL, params)
	// JetStream is derived from the same connection rather than opening a second
	// one, and it is part of the assembled client: a failure here discards the
	// connection rather than leaking it.
	if c.JetStream.Enabled {
		js, err := jetstream.New(nc)
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, err, "nats create jetstream context failed")
			nc.Close()
			return nil, errutil.Explain(err, "failed to create jetstream context")
		}
		conn.JetStream = js
	}
	return conn, nil
}

// newConn creates a NATS connection via the Driver bean — falling back to the
// bundled [DefaultDriver] when no company Driver bean is present — which owns the
// raw connection assembly (options/auth/TLS + nats.Connect) and returns the
// connection already wrapped (identity, governance, and the JetStream context
// when enabled). Connection-layer events (async errors, disconnect,
// reconnect, close) are bridged into go-spring's log by the driver's handlers so
// they show up alongside app logs.
//
// center is the governance center the container injects — the family's sole
// injection point; the ctor reads the resilience and fault authorities from it
// and bundles them into the [cloud.ClientParams] it hands the driver, which passes it to
// [NewConn] — so the connection is assembled complete in one step, with the zero
// bundle degrading to an observed-only, loudly-unmanaged executor.
func newConn(ctx *gs.ContextProvider, name string, c Config, d Driver,
	center *governance.Center) (*Conn, error) {
	// The connection's identity rides on a context derived here: every line
	// below carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.String("url", c.URL))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.String("name", c.Name),
			log.Msg("creating nats connection"),
		}
	})

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	conn, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		log.Errorf(cctx, log.TagAppDef, err, "nats: create client failed")
		return nil, errutil.Explain(err, "failed to create nats client: %s", c.URL)
	}
	// The Driver returned the connection complete — identity, governance and (when
	// enabled) JetStream all applied while it was built. There is no Init hook and
	// nothing else runs after this — the bean is complete when this ctor returns.
	// Fail fast (opt-in, e.g. ping=true): confirm the live connection state
	// before publishing the bean, so a connection that died between the dial and
	// the end of assembly surfaces during boot rather than on the first publish.
	// HealthCheck goes straight to the bare client on purpose: it is a
	// connectivity check, not business traffic, so it must not open a span or
	// spend limiter/breaker budget. A failure abandons the connection, so release
	// what was just applied. With ping unset the check is skipped and a dropped
	// connection only surfaces on first use.
	if c.Ping {
		if err := HealthCheck(ctx.Context, conn); err != nil {
			log.Errorf(cctx, log.TagAppDef, err, "nats: check connectivity at startup failed")
			_ = conn.Close()
			return nil, errutil.Explain(err, "nats: startup connectivity check failed: %s", c.URL)
		}
	}
	log.Infof(cctx, log.TagAppDef, "create nats connection success")
	return conn, nil
}
