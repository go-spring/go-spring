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

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/chain"

	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/log"
)

// LuohuaResilienceDriverName is the name this backend answers to in the
// governance document.
const LuohuaResilienceDriverName = "luohua"

// luohuaResilienceDriver is the luohua governance backend: it does NOT invent a
// new resilience standard — it reuses the bundled "default" Driver (the
// policy/breaker engine) via [resilience.NewDefaultDriver], then wraps the
// resulting executor with a luohua verification flavor. A fleet sets
// spring.resilience.driver=luohua once in the rules document and every governed call then carries an
// observable luohua marker, so a mis-wired backend is discoverable instead of
// silently passing through — the same "default + observable company flavor"
// shape the redigo [RedisDriver] layers on its pools.
type luohuaResilienceDriver struct {
	// limiter is the cluster rate-limit store the container holds, if any,
	// passed through so a luohua fleet keeps whatever sharing breadth the rest
	// of the process has (a Redis store, typically, which is what carries a
	// budget across replicas). It is nullable, and nil is the ordinary case: no
	// backend starter contributed a store, so the bundled engine gives each
	// executor a private one — one budget per service label all the same.
	limiter ratelimit.ClusterLimiter
}

// NewExecutor builds a luohua-flavored [chain.Executor] for service on top
// of the bundled engine.
func (d luohuaResilienceDriver) NewExecutor(service string, p resilience.Policy) (chain.Executor, error) {
	inner, err := resilience.NewDefaultDriver(d.limiter).NewExecutor(service, p)
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

func (e *luohuaExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	// Name the service this executor guards; a layer below (or a direct user that
	// left it blank) then has the call's identity without having to know it.
	if op.Service == "" {
		op.Service = e.service
	}
	// luohua verification flavor: a per-call trace tagged luohua/governance, so
	// a process where spring.resilience.driver=luohua is in effect is observable in logs.
	// This is the company hook point — a real luohua company would hang its own
	// policy/abort/audit logic here.
	log.Debug(ctx, log.TagAppDef, "luohua/governance", func() []log.Field {
		return []log.Field{log.String("service", e.service)}
	})
	return e.inner.Execute(ctx, op, rec, fn)
}

// SetPolicySource forwards the manager to the inner executor, so the luohua
// marker does not stand between the manager and the executor that follows its
// config. Forwarding it is not optional: an inner executor that never receives
// its source keeps the construction-time policy for every call, which means
// per-rule matching and hot reload are silently off for
// spring.resilience.driver=luohua — the marker would be the only reason.
func (e *luohuaExecutor) SetPolicySource(src resilience.PolicySource) {
	if s, ok := e.inner.(resilience.PolicySourceSetter); ok {
		s.SetPolicySource(src)
	}
}

func (e *luohuaExecutor) Close() error { return e.inner.Close() }
