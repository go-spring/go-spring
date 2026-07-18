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

package StarterRabbitMQ

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {

	// Register multiple RabbitMQ connections as a group.
	// Each instance is created according to the configuration in "${spring.rabbitmq.instances}".
	// This allows defining multiple RabbitMQ connections dynamically.
	gs.Group("${spring.rabbitmq.instances}", newClient, destroyClient)
}

// newClient dials RabbitMQ. amqp.Dial/DialConfig perform the TCP + AMQP
// handshake synchronously, so a bad URL, wrong credentials or TLS mismatch
// fail fast at startup rather than surfacing on the first channel/publish.
// Once the connection is up a probe channel is opened and closed to confirm
// the AMQP layer is usable, then close/block notifiers are bridged into
// go-spring's log so broker-driven events land alongside app logs.
func newClient(c Config) (*amqp.Connection, error) {
	ctx := context.Background()

	useTLS := c.TLS.Enabled || strings.HasPrefix(strings.ToLower(c.URL), "amqps://")

	var (
		conn *amqp.Connection
		err  error
	)
	if useTLS || c.Heartbeat > 0 || c.Vhost != "" {
		cfg := amqp.Config{
			Vhost:     c.Vhost,
			Heartbeat: c.Heartbeat,
		}
		if useTLS {
			tc, terr := tlsConfig(c.TLS)
			if terr != nil {
				return nil, terr
			}
			cfg.TLSClientConfig = tc
		}
		conn, err = amqp.DialConfig(c.URL, cfg)
	} else {
		conn, err = amqp.Dial(c.URL)
	}
	if err != nil {
		return nil, errutil.Explain(err, "failed to dial rabbitmq: %s", c.URL)
	}

	// Confirm the AMQP channel layer is usable, not just the TCP handshake.
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, errutil.Explain(err, "failed to open probe channel: %s", c.URL)
	}
	_ = ch.Close()

	// Bridge connection-level events into go-spring's log. NotifyClose fires
	// once when the connection tears down (server-initiated or network drop);
	// NotifyBlocked fires whenever the broker throttles the publisher due to
	// resource alarms. Both channels are closed by amqp091 on connection
	// shutdown, so the goroutines exit naturally without leaking.
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))
	blockCh := conn.NotifyBlocked(make(chan amqp.Blocking, 1))
	go func() {
		for e := range closeCh {
			if e == nil {
				log.Infof(ctx, log.TagAppDef, "rabbitmq connection closed: %s", c.URL)
				continue
			}
			log.Warnf(ctx, log.TagAppDef, "rabbitmq connection closed: code=%d reason=%q server=%t recover=%t",
				e.Code, e.Reason, e.Server, e.Recover)
		}
	}()
	go func() {
		for b := range blockCh {
			if b.Active {
				log.Warnf(ctx, log.TagAppDef, "rabbitmq connection blocked: %s", b.Reason)
			} else {
				log.Infof(ctx, log.TagAppDef, "rabbitmq connection unblocked")
			}
		}
	}()

	return conn, nil
}

// tlsConfig assembles a *tls.Config from optional CA / client cert files.
// Written to mirror starter-kafka's tlsConfig so the two starters read the
// same way, with the extra ServerName knob AMQPS often needs.
func tlsConfig(c TLSConfig) (*tls.Config, error) {
	tc := &tls.Config{
		InsecureSkipVerify: c.InsecureSkipVerify,
		ServerName:         c.ServerName,
	}
	if c.CACert != "" {
		pem, err := os.ReadFile(c.CACert)
		if err != nil {
			return nil, errutil.Explain(err, "failed to read rabbitmq tls ca-cert: %s", c.CACert)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("failed to parse rabbitmq tls ca-cert: %s", c.CACert)
		}
		tc.RootCAs = pool
	}
	if c.ClientCert != "" || c.ClientKey != "" {
		cert, err := tls.LoadX509KeyPair(c.ClientCert, c.ClientKey)
		if err != nil {
			return nil, errutil.Explain(err, "failed to load rabbitmq tls client cert/key")
		}
		tc.Certificates = []tls.Certificate{cert}
	}
	return tc, nil
}

// destroyClient closes the RabbitMQ connection. amqp091 closes the notifier
// channels as part of Close, which drains the log-bridging goroutines.
func destroyClient(conn *amqp.Connection) error {
	return conn.Close()
}
