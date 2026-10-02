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
	"context"
	"sync/atomic"

	"go-spring.org/cloud/traffic"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// loadTestAttribute is the span attribute marking a request as synthetic load.
const loadTestAttribute = "load_test"

// loadTestPropagator is the application's load-test convention, installed once
// at wiring time by [SetLoadTestPropagator]. It is a package handle rather than
// a field because the tracer provider — and with it this processor — is built
// during the gs prepare phase, before any bean exists, so no constructor could
// receive it. Until a hook installs one, the processor reads go-spring's
// default; spans only start after wiring, so the window is not observable.
var loadTestPropagator atomic.Pointer[traffic.Propagator]

// SetLoadTestPropagator installs p as the convention this package's span
// processor asks. A nil p restores go-spring's default. Call it at wiring time.
func SetLoadTestPropagator(p traffic.Propagator) {
	if p == nil {
		loadTestPropagator.Store(nil)
		return
	}
	loadTestPropagator.Store(&p)
}

func currentLoadTestPropagator() traffic.Propagator {
	if p := loadTestPropagator.Load(); p != nil {
		return *p
	}
	// DefaultBinding is complete, so this cannot fail.
	p, _ := traffic.NewDefaultPropagator(traffic.DefaultBinding())
	return p
}

// loadTestProcessor tags every span of a load-test request, so synthetic
// traffic can be told apart from real traffic in traces and metrics.
//
// It exists because the marker was invisible to telemetry. The load-test flag
// is consumed across the framework -- fault injection, every entry middleware,
// the messaging clients -- but nothing recorded it, so a load-test run reached
// the same dashboards and alerts as production traffic and looked identical.
//
// It asks the installed [traffic.Propagator] rather than reading the context
// marker directly, on purpose: a process that re-bases the convention gets the
// answer of its own convention, and this stays bound to the seam instead of to
// go-spring's default implementation of it.
//
// This is a read, not an action. A propagator carries the flag and never
// acts on it by design; deciding what a load-test request should DO (shadow
// table, isolated breaker) is the application's business. Recording a fact the
// framework already holds is a different matter -- it is what makes the
// observation complete.
type loadTestProcessor struct{}

// OnStart implements sdktrace.SpanProcessor.
func (loadTestProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if currentLoadTestPropagator().IsLoadTest(parent) {
		s.SetAttributes(attribute.Bool(loadTestAttribute, true))
	}
}

// OnEnd implements sdktrace.SpanProcessor.
func (loadTestProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

// Shutdown implements sdktrace.SpanProcessor. It holds no resources, so there is
// nothing to release.
func (loadTestProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush implements sdktrace.SpanProcessor. It holds no resources, so there
// is nothing to flush.
func (loadTestProcessor) ForceFlush(context.Context) error { return nil }

// LoadTestProcessor returns the SpanProcessor that tags load-test spans with
// load_test=true.
//
// [NewTracerProvider] registers one automatically. It is exported for the same
// reason observability.SpanAttributesProcessor is: an application that builds
// its own TracerProvider must register the processors itself, or it silently
// gets different telemetry from the framework's own setup.
func LoadTestProcessor() sdktrace.SpanProcessor { return loadTestProcessor{} }
