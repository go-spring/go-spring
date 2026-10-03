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
	"go-spring.org/cloud/chain"

	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
)

// LuohuaResilienceDriverName is the name this backend answers to in the
// governance document.
const LuohuaResilienceDriverName = "luohua"

// luohuaResilienceDriver is the luohua governance backend: it does NOT invent a
// new resilience standard — it reuses the bundled "default" Driver (the
// policy/breaker engine) via [resilience.NewDefaultDriver], then wraps the
// resulting executor with a luohua verification flavor. A fleet sets
// spring.governance.driver=luohua once in the rules document and every governed call then carries an
// observable luohua marker, so a mis-wired backend is discoverable instead of
// silently passing through — the same "default + observable company flavor"
// shape the redigo [RedisDriver] layers on its pools.
type luohuaResilienceDriver struct {
	// counters is the rate-limit counter store the container holds, if any,
	// passed through so a luohua fleet keeps whatever sharing breadth the rest
	// of the process has (a Redis store, typically, which is what carries a
	// budget across replicas). It is nullable, and nil is the ordinary case: no
	// backend starter contributed a store, so the bundled engine gives each
	// executor a private one — one budget per service label all the same.
	counters resilience.Counters
}

// NewClientExecutor builds a luohua-flavored [chain.Executor] for service on top
// of the bundled engine.
func (d luohuaResilienceDriver) NewClientExecutor(service string, p resilience.ClientPolicy) (chain.Executor, error) {
	inner, err := resilience.NewDefaultDriver(d.counters).NewClientExecutor(service, p)
	if err != nil {
		return nil, err
	}
	return &luohuaExecutor{inner: inner, service: service}, nil
}

// luohuaExecutor wraps the inner (default) executor and stamps each governed
// call with the luohua verification marker before delegating.
type luohuaExecutor struct {
	inner   chain.Executor
	service string
}

func (e *luohuaExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	// luohua verification flavor: a per-call trace tagged luohua/governance, so
	// a process where spring.governance.driver=luohua is in effect is observable in logs.
	// This is the company hook point — a real luohua company would hang its own
	// policy/abort/audit logic here.
	log.Debugf(ctx, log.TagAppDef, "luohua/governance service=%s", e.service)
	return e.inner.Execute(ctx, fn)
}

// SetBreakerEventListener forwards to the inner executor so the breakers of the
// driver beneath luohua still emit state transitions. A wrapper that swallowed
// this capability would silently disable breaker observability for
// spring.governance.driver=luohua, since [resilience.ClientExecutorFor] attaches the observe
// listener through this handshake.
func (e *luohuaExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	if s, ok := e.inner.(resilience.BreakerEventListenerSetter); ok {
		s.SetBreakerEventListener(l)
	}
}

func (e *luohuaExecutor) Close() error { return e.inner.Close() }

// NewServerExecutor is the inbound counterpart of [luohuaResilienceDriver.NewClientExecutor]:
// it builds the bundled driver's inbound executor and wraps it with the same
// luohua marker, so spring.governance.driver=luohua flavors inbound governance too — a fleet
// whose only outbound calls were marked would leave the inbound half of its
// governance unobservable.
func (d luohuaResilienceDriver) NewServerExecutor(service string, p resilience.ServerPolicy) (chain.Executor, error) {
	inner, err := resilience.NewDefaultDriver(d.counters).NewServerExecutor(service, p)
	if err != nil {
		return nil, err
	}
	return &luohuaServerExecutor{inner: inner, service: service}, nil
}

// luohuaServerExecutor is the inbound twin of [luohuaExecutor]: the same
// marker, the same delegating contract, refreshed with the inbound model it was
// built from.
type luohuaServerExecutor struct {
	inner   chain.Executor
	service string
}

func (e *luohuaServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	log.Debugf(ctx, log.TagAppDef, "luohua/governance inbound service=%s", e.service)
	return e.inner.Execute(ctx, fn)
}

func (e *luohuaServerExecutor) SetBreakerEventListener(l resilience.BreakerEventListener) {
	if s, ok := e.inner.(resilience.BreakerEventListenerSetter); ok {
		s.SetBreakerEventListener(l)
	}
}

func (e *luohuaServerExecutor) Close() error { return e.inner.Close() }
