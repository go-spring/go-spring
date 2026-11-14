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
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// renderFields encodes fields the way the access log would, so a test can read
// what a line actually carries.
func renderFields(fields []log.Field) string {
	var buf bytes.Buffer
	log.EncodeFields(log.NewTextEncoder(&buf, " "), fields)
	return buf.String()
}

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

// TestSuccessQuiet pins the access-log levelling: it is the operation's own
// declaration that decides, which is how a high-frequency call (a cache GET, a
// health PING) stays out of the operator's log.
func TestSuccessQuiet(t *testing.T) {
	// Per-call detail: quiet.
	withDetail := dbOperation()
	withDetail.Observe.Log.QuietSuccess = true
	assert.That(t, successQuiet(withDetail)).True()
	// None: a line at Info.
	assert.That(t, successQuiet(dbOperation())).False()
}

// TestCallLabelsDoNotAlias proves the labels are built on a fresh slice: the
// attributes come off the context and are shared, so writing through them would
// corrupt a sibling call's labels.
func TestCallLabelsDoNotAlias(t *testing.T) {
	attrs := make([]attribute.KeyValue, 1, 4) // spare capacity: an in-place append would fit
	attrs[0] = attribute.String("db.system", "memcached")

	out := callLabels("memcached.cache", attrs, "ok", "")

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
	assert.Number(t, len(callLabels("memcached.cache", attrs, "error", "rate_limited"))).Equal(4)
}

// TestRetryFields proves the access line carries the runtime retry facts: the
// ceiling the plan allowed and the retries actually attempted. A call with no
// retry plan that never retried contributes nothing, so ordinary single-attempt
// lines stay unchanged.
func TestRetryFields(t *testing.T) {
	// A single attempt under no retry plan: unchanged line.
	plain := &chain.Recorder{}
	plain.NewRound()
	assert.That(t, retryFields(plain) == nil).True()

	// One attempt that was ALLOWED retries but did not need them.
	single := &chain.Recorder{}
	single.NewRound().MaxRetries = 2
	assert.String(t, renderFields(retryFields(single))).Equal("retry_max=2 retry_count=0")

	// Three rounds under a two-retry ceiling: the budget was fully used.
	used := &chain.Recorder{}
	for range 3 {
		used.NewRound()
	}
	used.LastRound().MaxRetries = 2
	assert.String(t, renderFields(retryFields(used))).Equal("retry_max=2 retry_count=2")
}

// TestSpanNameAndAttrs pins the naming: the span takes the operation's action
// word (falling back to the service label when it has none) and carries the
// declared attributes — plus the TARGET, which none of the protocol's own
// attributes says.
func TestSpanNameAndAttrs(t *testing.T) {
	w := &callEmitter{system: "memcached", service: "memcached.svc"}

	op := dbOperation()
	assert.String(t, w.spanName(op)).Equal("get")
	assert.String(t, w.spanName(chain.Operation{})).Equal("memcached.svc")

	declared := w.spanAttrs(op)
	assert.Number(t, len(declared)).Equal(3)
	assert.String(t, string(declared[0].Key)).Equal("peer.service")
	assert.String(t, declared[0].Value.AsString()).Equal("memcached.svc")
	assert.String(t, string(declared[1].Key)).Equal("db.system")

	// A declaration that names its own target wins over the emitter's label.
	named := op
	named.Service = "redis:cache"
	namedAttrs := w.spanAttrs(named)
	assert.String(t, namedAttrs[0].Value.AsString()).Equal("redis:cache")
}

// TestFactsAreWrittenBack proves the emitter writes the call's outcome onto the
// per-call Recorder, so the caller that built the declaration can read what
// happened without re-deriving it — the write-back half the chain now carries.
func TestFactsAreWrittenBack(t *testing.T) {
	ob := NewOperationObserver("memcached", "cache:1", NewOTelMetrics())

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

func (f fakeExecutor) Execute(ctx context.Context, op chain.Operation, rec *chain.Recorder, fn chain.RoundFunc) error {
	// A compliant executor records the attempt it made, even one it refuses.
	rec.NewRound().SetError(f.err)
	return f.err
}
func (fakeExecutor) Close() error { return nil }

// TestWrapClientExecutor_NilNextIsTerminal proves a nil next makes the
// observer the last layer: the call is fn itself, and with no governance loop
// below the observer records the call's single try as its own round.
func TestWrapClientExecutor_NilNextIsTerminal(t *testing.T) {
	exec := WrapClientExecutor(nil, "sys", "svc", NewOTelMetrics())
	assert.That(t, exec).NotNil()
	called := false
	rec := &chain.Recorder{}
	err := exec.Execute(context.Background(), dbOperation(), rec, func(context.Context, *chain.Round) error {
		called = true
		return nil
	})
	assert.That(t, err).Nil()
	assert.That(t, called).True()
	assert.That(t, len(rec.Rounds)).Equal(1)
}

// TestClassifyStatusAndOutcome proves the two keys answer two different
// questions, which is why they are separate: `status` is the ecosystem-wide axis
// (`ok`/`error`, the same words an inbound request uses), and
// `resilience.error_type` is what protection decided about the call.
//
// The round and call vocabularies differ on purpose: a round that simply ran
// still classifies a downstream failure, while the call level additionally
// resolves caller deadlines (context needed).
func TestClassifyStatusAndOutcome(t *testing.T) {
	type want struct {
		err     error
		status  string
		gate    chain.ErrorType // RoundErrorType: per round
		outcome chain.ErrorType // CallErrorType: per call
	}
	downstream := errutil.Explain(nil, "boom")
	cases := []want{
		{nil, "ok", "", ""},
		{chain.ErrRateLimited, "error", "rate_limited", "rate_limited"},
		{chain.ErrCircuitOpen, "error", "circuit_open", "circuit_open"},
		{chain.ErrBulkheadFull, "error", "bulkhead_full", "bulkhead_full"},
		{chain.ErrRetryExceeded, "error", "retry_exceeded", "retry_exceeded"},
		{context.DeadlineExceeded, "error", chain.ErrorTypeUnknown, chain.ErrorTypeUnknown},
		{fmt.Errorf("%w: %w", chain.ErrCallTimeout, context.DeadlineExceeded), "error", chain.ErrorTypeCallTimeout, chain.ErrorTypeCallTimeout},
		// A plain downstream failure is an error; no gate word applies, but the
		// CALL still has a reason.
		{downstream, "error", chain.ErrorTypeUnknown, chain.ErrorTypeUnknown},
		{chain.ConnectError(errors.New("refused")), "error", chain.ErrorTypeConnectFailed, chain.ErrorTypeConnectFailed},
		{chain.ResourceError(errors.New("pool exhausted")), "error", chain.ErrorTypeResourceExhausted, chain.ErrorTypeResourceExhausted},
		{context.Canceled, "error", chain.ErrorTypeContextCanceled, chain.ErrorTypeContextCanceled},
		// wrapped sentinels must still classify by errors.Is.
		{errors.Join(chain.ErrCircuitOpen, errutil.Explain(nil, "detail")), "error", "circuit_open", "circuit_open"},
	}
	for _, c := range cases {
		assert.That(t, chain.ClassifyStatus(c.err)).Equal(c.status)
		assert.That(t, chain.RoundErrorType(c.err)).Equal(c.gate)
		assert.That(t, chain.CallErrorType(context.Background(), c.err)).Equal(c.outcome)
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
		exec := WrapClientExecutor(fakeExecutor{err: err}, "redis", "svc", NewOTelMetrics())
		got := exec.Execute(context.Background(), dbOperation(), new(chain.Recorder), func(ctx context.Context, _ *chain.Round) error { return nil })
		assert.That(t, errors.Is(got, err)).True()
	}
	_ = chain.ErrBulkheadFull // keep import even if slice above changes
}
