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

// Package StarterConfigBus adds a configuration refresh bus on top of an
// existing NATS connection (starter-nats). Blank-importing this package
// registers a ConfigBus bean that subscribes to a refresh subject and re-runs
// the application-wide property refresh whenever a signal arrives, so a change
// broadcast once refreshes every instance in the fleet.
//
// It complements the remote config-center starters (starter-config-{nacos,
// etcd,consul}): those already refresh a single instance from their own watch,
// while the bus covers cross-instance broadcast and refreshes triggered from
// outside the config center. The bus carries refresh *signals* only — never
// configuration content, which stays with the config center or local files.
//
// Configure the transport by pointing spring.config.bus.nats-instance at a
// connection defined under spring.nats.instances.* (default instance name
// "config-bus").
package StarterConfigBus

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"

	StarterNats "go-spring.org/starter-nats"

	"go-spring.org/spring/gs"
	bushealth "go-spring.org/starter-config-bus/health"
)

var (
	// starterTag identifies logs emitted by the config bus starter.
	starterTag = log.RegisterAppTag("config_bus", "")
)

func init() {
	// Register the bus as a named root object so it is always created (wiring
	// the refresh listener regardless of whether anything else depends on it)
	// without its Rooter export colliding with the application's own default
	// Rooter. Inject it elsewhere via autowire:"configBus".
	//
	// The Condition gate means blank-importing the starter alone assembles
	// nothing: without any spring.config.bus.* property there is no bus, so the
	// starter can sit on the classpath of apps that never configure NATS.
	gs.Provide(&ConfigBus{}).
		Condition(gs.OnProperty("spring.config.bus")).
		Name("configBus").
		Init((*ConfigBus).Init).
		Destroy((*ConfigBus).Destroy).
		Export(gs.As[gs.Rooter]())

	// Contribute the subscriber-health indicator under the same gate. It reports
	// the subscription, not the NATS connection: a dropped connection is
	// starter-nats's own indicator to report (and to let an app opt out of via
	// that instance's health.enabled), while a lost subscription is invisible to
	// every other probe and is exactly what leaves an instance silently stuck on
	// stale configuration.
	gs.Provide(func(bus *ConfigBus) *health.Indicator {
		return bushealth.NewBusHealth("configBus", bus.Healthy)
	}, gs.TagArg("configBus")).
		Condition(gs.OnProperty("spring.config.bus")).
		Name("config-bus:configBus")
}

// RefreshEvent is the payload published on the bus. It carries only a hint of
// what changed, never the configuration content itself: subscribers always
// re-read from their own configured sources so the config center remains the
// single source of truth.
type RefreshEvent struct {
	// Prefix names the configuration namespace that changed (e.g. "db"). An
	// empty Prefix means a full-fleet refresh: every subscriber refreshes
	// regardless of its WatchPrefixes.
	Prefix string `json:"prefix,omitempty"`

	// Origin identifies the publisher, purely for observability. It is filled
	// from Config.Origin, which defaults to the host name.
	Origin string `json:"origin,omitempty"`
}

// ConfigBus broadcasts and receives configuration refresh signals over a shared
// NATS connection. On a received signal it triggers the application-wide
// property refresh, so a change published once reaches every subscribing
// instance — the Go equivalent of Spring Cloud Bus's refresh broadcast.
//
// Both directions are instrumented by starter-nats: PublishMsgContext emits a
// producer span and injects the trace context into the message header, and
// Consume extracts it, so one broadcast appears as a single trace spanning the
// publisher and every subscriber. On top of that the bus reports its own
// business outcome per event (see observe.go).
//
// The NATS connection is injected by instance name (spring.config.bus.nats-
// instance, default "config-bus"); define that instance under
// spring.nats.instances.* in the usual way.
type ConfigBus struct {
	Conn   *StarterNats.Conn `autowire:"${spring.config.bus.nats-instance:=config-bus}"`
	Config Config            `value:"${spring.config.bus}"`

	prefixes []string
	sub      *nats.Subscription
	origin   string

	// ins holds the metrics; refresh is the property-refresh entry point, held
	// as a field so tests can drive onMessage without a running application.
	ins     instruments
	refresh func(context.Context) error
}

// Init prepares the bus and starts its listener: it resolves the publisher
// identity, builds the metrics, and subscribes. It runs as the bean's init hook,
// after the NATS connection has been injected.
//
// The instruments are built here rather than at package init because the OTel
// global meter binds to the first provider installed, and starter-otel installs
// its own after this package's init.
func (b *ConfigBus) Init() error {
	b.ins = newInstruments()
	b.refresh = gs.RefreshProperties
	b.origin = b.Config.Origin
	if b.origin == "" {
		if host, err := os.Hostname(); err == nil {
			b.origin = host
		}
	}
	return b.subscribe()
}

// subscribe registers the refresh listener on the configured subject.
func (b *ConfigBus) subscribe() error {
	for p := range strings.SplitSeq(b.Config.WatchPrefixes, ",") {
		if p = strings.TrimSpace(p); p != "" {
			b.prefixes = append(b.prefixes, p)
		}
	}

	// An empty queue group makes this a plain subscription: every instance
	// sharing the subject receives every broadcast. Conn.Consume owns the
	// consumer span, so onMessage's ctx carries the publisher's trace.
	sub, err := b.Conn.Consume(context.Background(), b.Config.Subject, "",
		func(ctx context.Context, m *nats.Msg) error {
			return b.onMessage(ctx, m)
		})
	if err != nil {
		return errutil.Explain(err, "config bus: subscribe to %q failed", b.Config.Subject)
	}
	b.sub = sub
	log.Infof(context.Background(), starterTag,
		"config bus: subscribed subject=%s prefixes=%v origin=%s",
		b.Config.Subject, b.prefixes, b.origin)
	return nil
}

// onMessage handles one broadcast. It runs inside the consumer span opened by
// Conn.Consume, so ctx carries the publisher's trace and every log line joins
// it. Returns a non-nil error only when a refresh was attempted and failed, so
// the consumer span is marked failed for exactly that case.
func (b *ConfigBus) onMessage(ctx context.Context, m *nats.Msg) error {
	var ev RefreshEvent
	if len(m.Data) > 0 {
		if err := json.Unmarshal(m.Data, &ev); err != nil {
			b.record(ctx, outcomeMalformed, ev, 0, err)
			return nil
		}
	}
	if !b.shouldRefresh(ev.Prefix) {
		b.record(ctx, outcomeIgnored, ev, 0, nil)
		return nil
	}
	start := time.Now()
	if err := b.refresh(ctx); err != nil {
		b.record(ctx, outcomeRefreshError, ev, time.Since(start), err)
		return err
	}
	b.record(ctx, outcomeRefreshed, ev, time.Since(start), nil)
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
//
// ctx links the producer span to the caller's trace, so a refresh triggered from
// an HTTP endpoint shows up as part of that request rather than as an orphan.
func (b *ConfigBus) Publish(ctx context.Context, prefix string) error {
	data, err := json.Marshal(RefreshEvent{Prefix: prefix, Origin: b.origin})
	if err != nil {
		b.recordPublish(ctx, outcomePublishError, 0, err)
		return errutil.Explain(err, "config bus: marshal refresh event failed")
	}
	start := time.Now()
	err = b.Conn.PublishMsgContext(ctx, &nats.Msg{Subject: b.Config.Subject, Data: data})
	if err != nil {
		b.recordPublish(ctx, outcomePublishError, time.Since(start), err)
		return errutil.Explain(err, "config bus: publish to %q failed", b.Config.Subject)
	}
	b.recordPublish(ctx, outcomePublishOK, time.Since(start), nil)
	return nil
}

// Healthy reports whether the bus can still receive refresh events. It probes
// the subscription rather than the connection: a subscription survives a
// reconnect, so this is false only when the listener is genuinely dead — the
// case a connectivity check cannot see, and the one that leaves an instance on
// stale configuration in silence. Connection-level health belongs to
// starter-nats, which registers its own indicator per instance.
func (b *ConfigBus) Healthy() bool {
	return b.sub != nil && b.sub.IsValid()
}

// Destroy unsubscribes from the bus. It runs as the bean's destroy hook. The
// underlying NATS connection is owned by starter-nats and closed there.
func (b *ConfigBus) Destroy() error {
	if b.sub != nil {
		return b.sub.Unsubscribe()
	}
	return nil
}
