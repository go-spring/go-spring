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

package observability

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// dbOperation is a declared client operation in the db family: the vocabulary
// a gorm-style client would declare.
func dbOperation() Operation {
	return Operation{
		Name:   "get",
		Metric: "db.client",
		Attrs: []attribute.KeyValue{
			attribute.String("db.system", "memcached"),
			attribute.String("db.operation", "get"),
		},
	}
}

// TestSuccessQuiet pins the access-log levelling, including the case the
// refactor got wrong: an UNDECLARED call must stay at Debug, as it always was —
// otherwise every skipped operation (redigo's PING on every health probe) turns
// into an Info line, which is exactly the noise declaring nothing was meant to
// avoid.
func TestSuccessQuiet(t *testing.T) {
	// Undeclared: quiet, whatever the (zero) operation says.
	assert.That(t, successQuiet(false, Operation{})).True()
	// Declared with per-call detail: quiet.
	withDetail := dbOperation()
	withDetail.Detail = []attribute.KeyValue{attribute.String("db.statement", "k")}
	assert.That(t, successQuiet(true, withDetail)).True()
	// Declared with none: a line at Info.
	assert.That(t, successQuiet(true, dbOperation())).False()
}

// TestCallLabelsDoNotAlias proves the labels are built on a fresh slice: the
// attributes come off the context and are shared, so writing through them would
// corrupt a sibling call's labels.
func TestCallLabelsDoNotAlias(t *testing.T) {
	w := &wrappedClientExecutor{system: "memcached", service: "memcached.cache"}
	attrs := make([]attribute.KeyValue, 1, 4) // spare capacity: an in-place append would fit
	attrs[0] = attribute.String("db.system", "memcached")

	out := w.callLabels(attrs, "ok", "")

	assert.Number(t, len(attrs)).Equal(1)
	assert.Number(t, len(out)).Equal(3)
	assert.String(t, string(out[1].Key)).Equal("service")
	assert.String(t, string(out[2].Key)).Equal("status")

	// An outcome is an extra label, and an empty one is dropped rather than
	// emitted with a meaningless value.
	assert.Number(t, len(w.callLabels(attrs, "error", "rate_limited"))).Equal(4)
}

// TestSpanNameAndAttrs pins the naming split: the declared path names the span
// after the operation and carries its attributes, the undeclared path keeps the
// service label and this layer's own attributes.
func TestSpanNameAndAttrs(t *testing.T) {
	w := &wrappedClientExecutor{system: "memcached", service: "memcached.svc"}

	op := dbOperation()
	assert.String(t, w.spanName(op, true)).Equal("get")
	assert.String(t, w.spanName(op, false)).Equal("memcached.svc")

	declared := w.spanAttrs(op, true)
	assert.Number(t, len(declared)).Equal(2)
	assert.String(t, string(declared[0].Key)).Equal("db.system")

	fallback := w.spanAttrs(Operation{}, false)
	assert.Number(t, len(fallback)).Equal(2)
	assert.String(t, string(fallback[0].Key)).Equal("resilience.system")
}
