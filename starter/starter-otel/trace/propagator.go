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

package trace

import (
	"sort"
	"sync"

	"go.opentelemetry.io/otel/propagation"
)

// Text-map propagators are pluggable by name, mirroring the driver-registry
// idiom used for span/meter exporters (RegisterSpanExporter / RegisterMeterExporter):
// the W3C built-ins self-register at init, and a company can add its own — for
// example a propagator that carries its named business headers (X-Tenant, ...)
// — by calling RegisterPropagator from an init, then listing it in the
// ${spring.observability.trace.propagator} spec (see NewPropagator). Every
// transport (httpx, gin/echo/grpc) extracts/injects through the OTel process
// global, so a propagator registered here rides every inbound and outbound hop
// once it is composed into that global by setupTrace.
var (
	propMu sync.RWMutex
	propReg = make(map[string]propagation.TextMapPropagator)
)

// RegisterPropagator makes a text-map propagator available under name. It
// panics on an empty name, a nil propagator, or a duplicate — mirroring the
// driver-registry idiom elsewhere, so a mis-wired or duplicate registration
// fails loudly at init. The built-in names "tracecontext" and "baggage" are
// pre-registered at init and must not be re-registered.
func RegisterPropagator(name string, p propagation.TextMapPropagator) {
	if name == "" {
		panic("trace: register propagator with empty name")
	}
	if p == nil {
		panic("trace: register nil propagator: " + name)
	}
	propMu.Lock()
	defer propMu.Unlock()
	if _, ok := propReg[name]; ok {
		panic("trace: propagator already registered: " + name)
	}
	propReg[name] = p
}

func init() {
	RegisterPropagator("tracecontext", propagation.TraceContext{})
	RegisterPropagator("baggage", propagation.Baggage{})
}

func lookupPropagator(name string) (propagation.TextMapPropagator, bool) {
	propMu.RLock()
	defer propMu.RUnlock()
	p, ok := propReg[name]
	return p, ok
}

func propagatorNames() []string {
	propMu.RLock()
	defer propMu.RUnlock()
	names := make([]string, 0, len(propReg))
	for n := range propReg {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
