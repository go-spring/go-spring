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

package observe

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// dbOperation is a declared client operation in the db family: the vocabulary
// a gorm-style client would declare.
func dbOperation() chain.Operation {
	return chain.Operation{
		Request: chain.Request{Action: "get"},
		Observe: chain.Observe{
			Metric: chain.Metric{
				Name: "db.client",
				Attrs: []attribute.KeyValue{
					attribute.String("db.system", "memcached"),
					attribute.String("db.operation", "get"),
				},
			},
			Span: chain.Span{Attrs: []attribute.KeyValue{
				attribute.String("db.system", "memcached"),
				attribute.String("db.operation", "get"),
			}},
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
	assert.That(t, successQuiet(false, chain.Operation{})).True()
	// Declared with per-call detail: quiet.
	withDetail := dbOperation()
	withDetail.Observe.Log.QuietSuccess = true
	assert.That(t, successQuiet(true, withDetail)).True()
	// Declared with none: a line at Info.
	assert.That(t, successQuiet(true, dbOperation())).False()
}

// TestCallLabelsDoNotAlias proves the labels are built on a fresh slice: the
// attributes come off the context and are shared, so writing through them would
// corrupt a sibling call's labels.
func TestCallLabelsDoNotAlias(t *testing.T) {
	w := &callEmitter{system: "memcached", service: "memcached.cache"}
	attrs := make([]attribute.KeyValue, 1, 4) // spare capacity: an in-place append would fit
	attrs[0] = attribute.String("db.system", "memcached")

	out := w.callLabels("memcached.cache", attrs, "ok", "")

	assert.Number(t, len(attrs)).Equal(1)
	assert.Number(t, len(out)).Equal(3)
	assert.String(t, string(out[1].Key)).Equal("service")
	assert.String(t, string(out[2].Key)).Equal("status")
	// The service label is the RESOLVED target the caller passed, not a field
	// read back off the emitter: one resolver feeds the label, the log field and
	// the span, so they cannot disagree about where the call went.
	assert.String(t, out[1].Value.AsString()).Equal("memcached.cache")

	// An outcome is an extra label, and an empty one is dropped rather than
	// emitted with a meaningless value.
	assert.Number(t, len(w.callLabels("memcached.cache", attrs, "error", "rate_limited"))).Equal(4)
}

// TestSpanNameAndAttrs pins the naming split: the declared path names the span
// after the operation and carries its attributes — plus the TARGET, which none
// of the protocol's own attributes says — while the undeclared path keeps the
// service label and this layer's own attributes.
func TestSpanNameAndAttrs(t *testing.T) {
	w := &callEmitter{system: "memcached", service: "memcached.svc"}

	op := dbOperation()
	assert.String(t, w.spanName(op, true)).Equal("get")
	assert.String(t, w.spanName(op, false)).Equal("memcached.svc")

	declared := w.spanAttrs(op, true)
	assert.Number(t, len(declared)).Equal(3)
	assert.String(t, string(declared[0].Key)).Equal("peer.service")
	assert.String(t, declared[0].Value.AsString()).Equal("memcached.svc")
	assert.String(t, string(declared[1].Key)).Equal("db.system")

	// A declaration that names its own target wins over the emitter's label.
	named := op
	named.Service = "redis:cache"
	namedAttrs := w.spanAttrs(named, true)
	assert.String(t, namedAttrs[0].Value.AsString()).Equal("redis:cache")

	fallback := w.spanAttrs(chain.Operation{}, false)
	assert.Number(t, len(fallback)).Equal(2)
	assert.String(t, string(fallback[0].Key)).Equal("resilience.system")
}

// TestFactsAreWrittenBack proves the emitter writes the call's outcome onto the
// per-call Recorder, so the caller that built the declaration can read what
// happened without re-deriving it — the write-back half the chain now carries.
func TestFactsAreWrittenBack(t *testing.T) {
	ob := NewOperationObserver("memcached", "cache:1")

	// The caller owns the record and hands it in, exactly as it would to
	// [chain.Executor.Execute] — the no-governance call's one try IS the call,
	// and it lands on the record the caller passed.
	op := dbOperation()
	boom := errors.New("boom")
	rec := &chain.Recorder{}
	_ = ob.Observe(context.Background(), op, rec, func(context.Context) error { return boom })
	// The call's status and error live per round: one failed round, its error is
	// the one the round recorded.
	assert.Number(t, len(rec.Rounds)).Equal(1)
	assert.String(t, rec.Rounds[0].Status).Equal("error")
	assert.Error(t, rec.Rounds[0].Error).Is(boom)

	ok := dbOperation()
	okRec := &chain.Recorder{}
	_ = ob.Observe(context.Background(), ok, okRec, func(context.Context) error { return nil })
	// The response-direction facts: a no-governance call is its own single
	// round, ok on the shared axis.
	assert.Number(t, len(okRec.Rounds)).Equal(1)
	assert.String(t, okRec.Rounds[0].Status).Equal("ok")
	assert.That(t, okRec.Rounds[0].Duration >= 0).True()
}

// fakeExecutor returns a fixed error from Execute so a test can drive every
// status classification without wiring a real driver.
type fakeExecutor struct{ err error }

func (f fakeExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn func(context.Context) error) error {
	return f.err
}
func (fakeExecutor) Close() error { return nil }

func TestWrapClientExecutor_NilInnerReturnsNil(t *testing.T) {
	assert.That(t, WrapClientExecutor(nil, "sys", "svc")).Nil()
}

// TestClassifyStatusAndOutcome proves the two keys answer two different
// questions, which is why they are separate: `status` is the ecosystem-wide axis
// (`ok`/`error`, the same words an inbound request uses), and
// `resilience.error_type` is what protection decided about the call — empty unless
// it refused one or the call ran out of budget.
func TestClassifyStatusAndOutcome(t *testing.T) {
	cases := []struct {
		err     error
		status  string
		outcome string
	}{
		{nil, "ok", ""},
		{chain.ErrRateLimited, "error", "rate_limited"},
		{chain.ErrCircuitOpen, "error", "circuit_open"},
		{chain.ErrBulkheadFull, "error", "bulkhead_full"},
		{chain.ErrRetryExceeded, "error", "retry_exceeded"},
		{context.DeadlineExceeded, "error", "timeout"},
		// A plain downstream failure is an error, but nothing protection did.
		{errutil.Explain(nil, "boom"), "error", ""},
		// wrapped sentinels must still classify by errors.Is.
		{errors.Join(chain.ErrCircuitOpen, errutil.Explain(nil, "detail")), "error", "circuit_open"},
	}
	for _, c := range cases {
		assert.That(t, chain.ClassifyStatus(c.err)).Equal(c.status)
		assert.That(t, chain.ClassifyErrorType(c.err)).Equal(c.outcome)
	}
}

// TestWrapClientExecutor_PassesErrorThrough exercises every status end to end: the
// wrapper must return the inner error unchanged (no swallowing) while emitting
// signals. The OTel globals are no-ops here, so this only proves pass-through +
// no-panic; metric values are verified by the SDK-backed test below.
func TestWrapClientExecutor_PassesErrorThrough(t *testing.T) {
	for _, err := range []error{
		nil,
		chain.ErrRateLimited,
		chain.ErrCircuitOpen,
		chain.ErrBulkheadFull,
		context.DeadlineExceeded,
		errutil.Explain(nil, "downstream"),
	} {
		exec := WrapClientExecutor(fakeExecutor{err: err}, "redis", "svc")
		got := exec.Execute(context.Background(), chain.Operation{}, new(chain.Recorder), func(ctx context.Context) error { return nil })
		assert.That(t, errors.Is(got, err)).True()
	}
	_ = chain.ErrBulkheadFull // keep import even if slice above changes
}
