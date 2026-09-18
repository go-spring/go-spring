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

	"go-spring.org/cloud/governance/traffic"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// loadTestAttribute is the span attribute marking a request as synthetic load.
const loadTestAttribute = "load_test"

// loadTestProcessor tags every span of a load-test request, so synthetic
// traffic can be told apart from real traffic in traces and metrics.
//
// It exists because the marker was invisible to telemetry. traffic.IsLoadTest
// is consumed across the framework -- fault injection, every entry middleware,
// the messaging clients -- but nothing recorded it, so a load-test run reached
// the same dashboards and alerts as production traffic and looked identical.
//
// It reads the [traffic] contract rather than the canonical marker package on
// purpose: a company that installs its own predicate gets its own answer, and
// this stays bound to the contract instead of to go-spring's default
// implementation of it.
//
// This is a read, not an action. The marker package carries the flag and never
// acts on it by design; deciding what a load-test request should DO (shadow
// table, isolated breaker) is the application's business. Recording a fact the
// framework already holds is a different matter -- it is what makes the
// observation complete.
type loadTestProcessor struct{}

// OnStart implements sdktrace.SpanProcessor.
func (loadTestProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if traffic.IsLoadTest(parent) {
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
// reason [ContextAttributesProcessor] is: an application that builds its own
// TracerProvider must register the processors itself, or it silently gets
// different telemetry from the framework's own setup.
func LoadTestProcessor() sdktrace.SpanProcessor { return loadTestProcessor{} }
