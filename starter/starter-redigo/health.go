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

	"go-spring.org/cloud/actuator/health"
)

// HealthCheck dials one bare connection and PINGs it — the cheapest read-only
// round trip that proves the pool's target is reachable. It is the module's
// single health implementation: the Actuator probe ([NewClientHealth]) and the
// opt-in startup probe in the constructor both delegate to it.
//
// It uses Pool.Dial (a non-pooled dial) instead of Pool.Get: a conn borrowed
// via Get is returned to the idle pool on Close, and that happens before the
// pool's dial is wrapped with the instrumented Conn — so the stale raw conn
// would later be handed out with no instrumentation and silently bypass
// resilience. Dialing directly keeps it out of the pool.
func HealthCheck(ctx context.Context, p *Pool) error {
	conn, err := p.Dial()
	if err != nil {
		return err
	}
	_, pingErr := conn.Do("PING")
	_ = conn.Close()
	return pingErr
}

// NewClientHealth builds an indicator for a redigo connection pool. It is
// registered once per configured instance and exported as health.Indicator, so
// an application that also imports starter-actuator gets redigo readiness
// folded into /readiness with no extra wiring. The pool dials lazily, so the
// probe delegates to [HealthCheck], the module's single health implementation.
func NewClientHealth(name string, p *Pool) *health.Indicator {
	return &health.Indicator{Name: "redigo:" + name, Probe: func(ctx context.Context) error {
		return HealthCheck(ctx, p)
	}}
}
