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

package luohua

import (
	"context"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
)

// luohuaResilienceDriver is the luohua governance backend: it does NOT invent a
// new resilience standard — it reuses the bundled "default" Driver (the
// policy/breaker engine) via the public [resilience.GetDriver] seam, then wraps
// the resulting executor with a luohua verification flavor. A fleet sets
// govern.driver=luohua once in the rules document and every governed call then carries an
// observable luohua marker, so a mis-wired backend is discoverable instead of
// silently passing through — the same "default + observable company flavor"
// shape the redigo [RedisDriver] layers on its pools.
type luohuaResilienceDriver struct{}

// NewExecutor builds a luohua-flavored [resilience.Executor] on top of the
// bundled engine.
func (luohuaResilienceDriver) NewExecutor(p resilience.Policy) (resilience.Executor, error) {
	d, err := resilience.GetDriver("default")
	if err != nil {
		return nil, err
	}
	inner, err := d.NewExecutor(p)
	if err != nil {
		return nil, err
	}
	return &luohuaExecutor{inner: inner}, nil
}

func init() {
	// Registering the backend under the fleet-standard "luohua" name makes it
	// selectable by govern.driver=luohua. Like the bundled "default" (and
	// sentinel's blank-import registration) this is an init-time availability
	// registration, not a config-gated activation — the driver only governs when
	// the process actually selects it.
	resilience.RegisterDriver("luohua", luohuaResilienceDriver{})
}

// luohuaExecutor wraps the inner (default) executor and stamps each governed
// call with the luohua verification marker before delegating.
type luohuaExecutor struct {
	inner resilience.Executor
}

func (e *luohuaExecutor) Execute(ctx context.Context, resource string, fn func(context.Context) error) error {
	// luohua verification flavor: a per-call trace tagged luohua/governance, so
	// a process where govern.driver=luohua is in effect is observable in logs.
	// This is the company hook point — a real luohua company would hang its own
	// policy/abort/audit logic here.
	log.Debugf(ctx, log.TagAppDef, "luohua/governance resource=%s", resource)
	return e.inner.Execute(ctx, resource, fn)
}

// SetBreakerEventListener forwards to the inner executor so the breakers of the
// driver beneath luohua still emit state transitions. A wrapper that swallowed
// this capability would silently disable breaker observability for
// govern.driver=luohua, since [resilience.ExecutorFor] attaches the observe
// listener through this handshake.
func (e *luohuaExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	if s, ok := e.inner.(resilience.BreakerEventListenerSetter); ok {
		s.SetBreakerEventListener(l)
	}
}

func (e *luohuaExecutor) Refresh(p resilience.Policy) error { return e.inner.Refresh(p) }

func (e *luohuaExecutor) Close() error { return e.inner.Close() }
