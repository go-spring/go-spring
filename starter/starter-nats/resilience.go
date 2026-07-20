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
	"time"

	"github.com/nats-io/nats.go"
	"go-spring.org/spring/cloud/resilience"
)

// applyResilience builds an executor from the configured driver and attaches it
// to conn, unless resilience is disabled. This is the nats seam of
// stdlib/resilience: because nats exposes no reject-capable middleware (unlike
// redis.Hook or http.RoundTripper), the same backend-neutral Executor is driven
// through opt-in call-site guards (PublishGuarded/RequestGuarded) rather than a
// transparent interceptor. Only the adapter shape differs — the core is reused.
func applyResilience(c Config, conn *Conn) error {
	if !c.Resilience.Enabled {
		return nil
	}
	drv, err := resilience.MustGetDriver(c.Resilience.Driver)
	if err != nil {
		return err
	}
	exec, err := drv.NewExecutor(c.Resilience.policy())
	if err != nil {
		return err
	}
	conn.exec = exec
	conn.resource = resourceLabel(c)
	return nil
}

// resourceLabel derives a stable, human-readable resilience resource key for a
// connection, so limiter and breaker state is scoped per NATS instance rather
// than per subject. Name is preferred (it's set explicitly by the operator);
// URL is the natural fallback.
func resourceLabel(c Config) string {
	switch {
	case c.Name != "":
		return "nats:" + c.Name
	case c.URL != "":
		return "nats:" + c.URL
	default:
		return "nats"
	}
}

// guard routes call through the executor when one is attached, and otherwise
// runs it inline. Splitting this out keeps the guarded methods trivial and
// makes the pass-through / rejection paths independently testable without a
// live nats server.
func (c *Conn) guard(ctx context.Context, call func(context.Context) error) error {
	if c.exec == nil {
		return call(ctx)
	}
	return c.exec.Execute(ctx, c.resource, call)
}

// PublishGuarded publishes data on subj, routed through the resilience executor
// when Config.Resilience.Enabled is true. When resilience is disabled this
// behaves exactly like the embedded Publish, so enabling protection is a
// zero-code opt-in on the caller side. Uses a background context because
// nats.Conn.Publish takes no deadline; use RequestGuarded when a per-attempt
// timeout matters.
func (c *Conn) PublishGuarded(subj string, data []byte) error {
	return c.guard(context.Background(), func(context.Context) error {
		return c.Publish(subj, data)
	})
}

// RequestGuarded sends a request/reply on subj, routed through the resilience
// executor when Config.Resilience.Enabled is true. When resilience is disabled
// this behaves exactly like the embedded Request. On rejection (rate-limit or
// open circuit) the returned error is a resilience sentinel and the reply is
// nil; the underlying Request is never invoked.
func (c *Conn) RequestGuarded(ctx context.Context, subj string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	var reply *nats.Msg
	err := c.guard(ctx, func(context.Context) error {
		msg, rerr := c.Conn.Request(subj, data, timeout)
		if rerr != nil {
			return rerr
		}
		reply = msg
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}
