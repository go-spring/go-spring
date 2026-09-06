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
	"strings"
	"sync"

	"go-spring.org/cloud/governance/traffic/canonical"
	"go-spring.org/starter-otel/trace"
	"go.opentelemetry.io/otel/propagation"
)

// propagatorName is the name under which the luohua named-header propagator is
// registered into the starter-otel/trace registry (the G2 seam). It only runs
// once a user composes it into the fleet propagator via
// spring.observability.trace.propagator=w3c,luohua — registering a name nobody
// lists is inert.
const propagatorName = "luohua"

// headerKey is a per-header context key type, so each luohua business header
// (X-Tenant, X-User, ...) lives under its own context slot and never collides
// with other keys.
type headerKey string

// carryHeader is the single place that turns a context into a header value and
// back; the propagator and the log hook agree on this namespace.
func carryHeader(ctx context.Context, name string) string {
	if v, _ := ctx.Value(headerKey(name)).(string); v != "" {
		return v
	}
	return ""
}

func putCarriedHeader(ctx context.Context, name, value string) context.Context {
	return context.WithValue(ctx, headerKey(name), value)
}

// luohuaHeaders is the configured set of business headers the propagator
// carries, read live per request because configuration arrives at module-run
// time (after package init, where RegisterPropagator must run).
var luohuaHeaders = struct {
	sync.RWMutex
	names []string
}{}

func setCarriedHeaders(names []string) {
	luohuaHeaders.Lock()
	luohuaHeaders.names = names
	luohuaHeaders.Unlock()
}

// luohuaPropagator implements propagation.TextMapPropagator for luohua's named
// business headers. It stores each carried value in the context under
// [headerKey] and injects/extracts through the configured header names, so the
// company's vocabulary rides the OTel global propagator across httpx / gin /
// echo / grpc without any per-transport code.
type luohuaPropagator struct{}

// Inject writes every carried header value from ctx onto carrier.
func (luohuaPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	luohuaHeaders.RLock()
	defer luohuaHeaders.RUnlock()
	for _, name := range luohuaHeaders.names {
		if v := carryHeader(ctx, name); v != "" {
			carrier.Set(name, v)
		}
	}
}

// Extract reads every configured header from carrier into ctx.
func (luohuaPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	luohuaHeaders.RLock()
	defer luohuaHeaders.RUnlock()
	for _, name := range luohuaHeaders.names {
		if v := carrier.Get(name); v != "" {
			ctx = putCarriedHeader(ctx, name, v)
		}
	}
	return ctx
}

// Fields reports the configured header names, so trace tooling knows what the
// propagator touches.
func (luohuaPropagator) Fields() []string {
	luohuaHeaders.RLock()
	defer luohuaHeaders.RUnlock()
	out := make([]string, len(luohuaHeaders.names))
	copy(out, luohuaHeaders.names)
	return out
}

func init() {
	trace.RegisterPropagator(propagatorName, luohuaPropagator{})
}

// applyPropagate re-bases the wire vocabulary onto luohua's: it overrides the
// load-test marker header (the G1 seam) when configured, and arms the named
// business headers the propagator carries.
func applyPropagate(c PropagateConfig) error {
	if c.LoadTestHeader != "" {
		canonical.HeaderLoadTest = c.LoadTestHeader
		// gRPC metadata keys must be lowercase; derive from the HTTP header so
		// the two stay in lockstep for the same convention.
		canonical.MetaKeyLoadTest = strings.ToLower(c.LoadTestHeader)
	}
	if len(c.Headers) > 0 {
		setCarriedHeaders(c.Headers)
	}
	return nil
}
