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

package StarterConfigBus

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/nats-io/nats.go"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"

	StarterNats "go-spring.org/starter-nats"
)

// RefreshEvent is the payload published on the bus. It carries only a hint of
// what changed, never the configuration content itself: subscribers always
// re-read from their own configured sources so the config center remains the
// single source of truth.
type RefreshEvent struct {
	// Prefix names the configuration namespace that changed (e.g. "db"). An
	// empty Prefix means a full-fleet refresh: every subscriber refreshes
	// regardless of its WatchPrefixes.
	Prefix string `json:"prefix,omitempty"`

	// Origin optionally identifies the publisher, purely for observability.
	Origin string `json:"origin,omitempty"`
}

// ConfigBus broadcasts and receives configuration refresh signals over a shared
// NATS connection. On a received signal it triggers the application-wide
// property refresh, so a change published once reaches every subscribing
// instance — the Go equivalent of Spring Cloud Bus's refresh broadcast.
//
// The NATS connection is injected by instance name (spring.config.bus.nats-
// instance, default "config-bus"); define that instance under
// spring.nats.instances.* in the usual way.
type ConfigBus struct {
	Conn      *StarterNats.Conn       `autowire:"${spring.config.bus.nats-instance:=config-bus}"`
	Refresher *gs.PropertiesRefresher `autowire:""`
	Config    Config                  `value:"${spring.config.bus}"`

	prefixes []string
	sub      *nats.Subscription
}

// subscribe registers the refresh listener on the configured subject. It runs
// as the bean's init hook, after the NATS connection and refresher have been
// injected.
func (b *ConfigBus) subscribe() error {
	for p := range strings.SplitSeq(b.Config.WatchPrefixes, ",") {
		if p = strings.TrimSpace(p); p != "" {
			b.prefixes = append(b.prefixes, p)
		}
	}

	sub, err := b.Conn.Subscribe(b.Config.Subject, func(m *nats.Msg) {
		var ev RefreshEvent
		if len(m.Data) > 0 {
			if err := json.Unmarshal(m.Data, &ev); err != nil {
				log.Warnf(context.Background(), log.TagAppDef,
					"config bus: ignoring malformed refresh event: %v", err)
				return
			}
		}
		if !b.shouldRefresh(ev.Prefix) {
			return
		}
		if err := b.Refresher.RefreshProperties(); err != nil {
			log.Errorf(context.Background(), log.TagAppDef,
				"config bus: property refresh failed: %v", err)
			return
		}
		log.Infof(context.Background(), log.TagAppDef,
			"config bus: refreshed properties on event (prefix=%q origin=%q)", ev.Prefix, ev.Origin)
	})
	if err != nil {
		return errutil.Explain(err, "config bus: subscribe to %q failed", b.Config.Subject)
	}
	b.sub = sub
	return nil
}

// shouldRefresh decides whether a broadcast with the given prefix applies to
// this instance. A full-fleet broadcast (empty prefix) and an instance with no
// configured prefixes both always refresh; otherwise the event applies when its
// prefix overlaps one of the watched prefixes in either direction (so a "db"
// watcher reacts to a "db.pool" change and vice versa).
func (b *ConfigBus) shouldRefresh(prefix string) bool {
	if prefix == "" || len(b.prefixes) == 0 {
		return true
	}
	for _, p := range b.prefixes {
		if strings.HasPrefix(prefix, p) || strings.HasPrefix(p, prefix) {
			return true
		}
	}
	return false
}

// Publish broadcasts a refresh signal to the fleet. An empty prefix requests a
// full refresh from every subscriber; a non-empty prefix lets prefix-scoped
// subscribers opt out. Call it from application code or a management endpoint to
// force a coordinated refresh (e.g. after a change that the config center's own
// watch does not observe).
func (b *ConfigBus) Publish(prefix string) error {
	data, err := json.Marshal(RefreshEvent{Prefix: prefix})
	if err != nil {
		return errutil.Explain(err, "config bus: marshal refresh event failed")
	}
	if err := b.Conn.Publish(b.Config.Subject, data); err != nil {
		return errutil.Explain(err, "config bus: publish to %q failed", b.Config.Subject)
	}
	return nil
}

// close unsubscribes from the bus. It runs as the bean's destroy hook. The
// underlying NATS connection is owned by starter-nats and closed there.
func (b *ConfigBus) close() error {
	if b.sub != nil {
		return b.sub.Unsubscribe()
	}
	return nil
}
