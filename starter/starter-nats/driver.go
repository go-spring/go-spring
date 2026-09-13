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
// TLS + nats.Connect). Unlike the redis / kafka drivers, DefaultDriver returns
// the raw *nats.Conn — the observe observers, JetStream derivation and resilience
// executor are the starter's lifecycle concerns (see newConn below), not the
// driver's.
package StarterNats

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel/trace"
)

// Driver interface defines how to create a NATS connection (a *nats.Conn). It is
// an OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterNats.Driver); when none is present,
// starter-nats falls back to the bundled [DefaultDriver] inside connection
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every connection under
// ${spring.nats} is built through it, and per-instance differences are expressed
// through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*nats.Conn, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient dials NATS from the provided configuration. It owns the raw
// connection assembly — the name/reconnect/timeout options, the async
// error/disconnect/reconnect/close logging handlers, the user/token/creds/nkey
// auth and TLS — but not the observe observers, the JetStream context, the
// resilience wiring or the *Conn wrapper, which are the starter's lifecycle
// concerns (see newConn below).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*nats.Conn, error) {
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
			log.Errorf(ctx, log.TagAppDef, "nats async error on %q: %v", subj, err)
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Warnf(ctx, log.TagAppDef, "nats disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			log.Infof(ctx, log.TagAppDef, "nats reconnected to %q", nc.ConnectedUrl())
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			log.Infof(ctx, log.TagAppDef, "nats connection closed")
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
			log.Errorf(ctx, log.TagAppDef, "nats: build TLS failed: %v", err)
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
		log.Errorf(ctx, log.TagAppDef, "nats: connect failed url=%s: %v", c.URL, err)
		return nil, errutil.Explain(err, "failed to connect nats: %s", c.URL)
	}
	return nc, nil
}

// newConn creates a NATS connection via the Driver bean — falling back to the
// bundled [DefaultDriver] when no company Driver bean is present — which owns the
// raw connection assembly (options/auth/TLS + nats.Connect). After the connection
// is built it is wrapped into a *Conn: the observe observers are attached, the
// JetStream context is derived when enabled, and the resilience executor is
// wired. Connection-layer events (async errors, disconnect, reconnect, close) are
// bridged into go-spring's log by the driver's handlers so they show up alongside
// app logs.
func newConn(ctx *gs.ContextProvider, name string, c Config, d Driver) (*Conn, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating nats connection, url=%s name=%s", c.URL, c.Name)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	nc, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "nats: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create nats client: %s", c.URL)
	}

	conn := &Conn{Conn: nc}
	// Attach the instrumentation (trace+metric+log, see observe.go) for
	// publishes and consumes.
	conn.pubObs = newObserver(trace.SpanKindProducer)
	conn.subObs = newObserver(trace.SpanKindConsumer)
	if c.JetStream.Enabled {
		js, err := jetstream.New(nc)
		if err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "nats: create jetstream context failed: %v", err)
			nc.Close()
			return nil, errutil.Explain(err, "failed to create jetstream context")
		}
		conn.JetStream = js
	}
	if err := applyResilience(c, conn, resilience.ResourceLabel("nats", c.Name, c.URL)); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "nats: resilience setup failed: %v", err)
		nc.Close()
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "nats connection initialized, url=%s", c.URL)
	return conn, nil
}
