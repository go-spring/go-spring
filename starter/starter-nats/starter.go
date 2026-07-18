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

package StarterNats

import (
	"context"
	"crypto/tls"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// Conn wraps a NATS connection together with an optional JetStream context.
// The embedded *nats.Conn lets callers use Publish/Subscribe/Request directly on
// the bean; JetStream is non-nil only when jetstream.enabled is set, since it is
// derived from the same connection rather than opening a second one.
type Conn struct {
	*nats.Conn
	JetStream jetstream.JetStream
}

// Healthy reports whether the connection is currently established. It reflects
// the live state of the auto-reconnecting client, so callers (health probes,
// readiness endpoints) can query it at any time rather than relying only on the
// connection-event logs.
func (c *Conn) Healthy() bool {
	return c.Conn != nil && c.Conn.IsConnected()
}

func init() {
	// Register multiple NATS connections as a group.
	// Each instance is created according to the configuration in
	// "${spring.nats.instances}", allowing multiple connections dynamically.
	gs.Group("${spring.nats.instances}", newConn, destroyConn)
}

// newConn dials NATS and, when configured, derives a JetStream context from the
// same connection. Connection-layer events (async errors, disconnect, reconnect,
// close) are bridged into go-spring's log so they show up alongside app logs.
func newConn(c Config) (*Conn, error) {
	ctx := context.Background()

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
		if c.TLS.InsecureSkipVerify {
			opts = append(opts, nats.Secure(&tls.Config{InsecureSkipVerify: true}))
		} else {
			opts = append(opts, nats.Secure())
		}
		if c.TLS.CAFile != "" {
			opts = append(opts, nats.RootCAs(c.TLS.CAFile))
		}
		if c.TLS.CertFile != "" || c.TLS.KeyFile != "" {
			opts = append(opts, nats.ClientCert(c.TLS.CertFile, c.TLS.KeyFile))
		}
	}

	nc, err := nats.Connect(c.URL, opts...)
	if err != nil {
		return nil, errutil.Explain(err, "failed to connect nats: %s", c.URL)
	}

	conn := &Conn{Conn: nc}
	if c.JetStream.Enabled {
		js, err := jetstream.New(nc)
		if err != nil {
			nc.Close()
			return nil, errutil.Explain(err, "failed to create jetstream context")
		}
		conn.JetStream = js
	}
	return conn, nil
}

// destroyConn drains the connection, letting in-flight subscriptions finish
// before the underlying socket is closed. Drain closes the connection when done.
func destroyConn(conn *Conn) error {
	return conn.Drain()
}
