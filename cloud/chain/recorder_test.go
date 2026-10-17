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

package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// A nil Recorder is the "record nothing" case: every method must tolerate it, so
// a layer can record unconditionally and a caller can pass nil to decline.
func TestNilRecorderIsSafe(t *testing.T) {
	var rec *chain.Recorder

	rec.SetFault(&chain.Injection{Kind: "timeout"})
	rec.AddLog(log.String("cache", "hit"))
	rec.AddSpan(attribute.String("cache.result", "hit"))
	rec.AddMetric(attribute.String("cache.result", "hit"))
	rec.AddRound(chain.Round{Start: time.Now(), Duration: time.Millisecond, Status: "ok"})
	rec.Reset()

	assert.That(t, rec.Rounds() == nil).True("a nil recorder records no rounds")
	_, ok := rec.LastRound()
	assert.That(t, ok).False("a nil recorder has no last round")
}

// The produced observability block is the write-back counterpart of the declared
// one: three groups, appended the same way, read by the emitter. Reset drops it
// with everything else, so a pooled Recorder carries no call's business fields
// into the next.
func TestProducedObservabilityIsKeptPerCall(t *testing.T) {
	rec := &chain.Recorder{}
	rec.AddLog(log.String("cache", "hit"))
	rec.AddSpan(attribute.String("cache.result", "hit"))
	rec.AddMetric(attribute.String("cache.result", "hit"))

	assert.Number(t, len(rec.Observed.Log)).Equal(1)
	assert.Number(t, len(rec.Observed.Span)).Equal(1)
	assert.Number(t, len(rec.Observed.Metric)).Equal(1)
	assert.String(t, rec.Observed.Span[0].Value.AsString()).Equal("hit")

	rec.Reset()

	assert.Number(t, len(rec.Observed.Log)).Equal(0)
	assert.Number(t, len(rec.Observed.Span)).Equal(0)
	assert.Number(t, len(rec.Observed.Metric)).Equal(0)
}

// The fire is a CALL-level fact, not a per-round one: the fault layer sets it on
// the record, the last fire wins, and Reset clears it so a pooled Recorder
// carries no call's fire into the next.
func TestSetFaultIsCallLevel(t *testing.T) {
	rec := &chain.Recorder{}
	assert.That(t, rec.Fault == nil).True("no fire yet")

	rec.SetFault(&chain.Injection{Kind: "reset", Rule: "redis:cache[0]"})
	assert.String(t, rec.Fault.Kind).Equal("reset")
	assert.String(t, rec.Fault.Rule).Equal("redis:cache[0]")

	// A later attempt's fire replaces the earlier one.
	rec.SetFault(&chain.Injection{Kind: "timeout"})
	assert.String(t, rec.Fault.Kind).Equal("timeout")

	rec.Reset()
	assert.That(t, rec.Fault == nil).True("Reset drops the fire")
}

// AddRound appends what the loop measured, and the rounds stay independent of the
// call-level fire.
func TestAddRoundAppendsTheRounds(t *testing.T) {
	rec := &chain.Recorder{}
	start := time.Now()

	rec.AddRound(chain.Round{Start: start, Duration: 7 * time.Millisecond, Status: "ok"})
	rec.AddRound(chain.Round{Start: time.Now(), Duration: time.Millisecond, Status: "error",
		ErrorType: "circuit_open", Err: chanErr()})

	rounds := rec.Rounds()
	assert.Number(t, len(rounds)).Equal(2)
	assert.That(t, rounds[0].Duration).Equal(7 * time.Millisecond)
	assert.That(t, rounds[0].Status).Equal("ok")
	assert.String(t, rounds[1].ErrorType).Equal("circuit_open")

	last, ok := rec.LastRound()
	assert.That(t, ok).True("the last round is the one just appended")
	assert.That(t, errors.Is(last.Err, chanErr())).False("each round carries its own error") // same text, different value
	assert.String(t, last.Err.Error()).Equal("downstream refused")
}

// A round the chain refused before touching the downstream is still a round: it
// carries the gate's verdict, which is how "nothing ran" is told apart from "ran
// and failed".
func TestRefusedRoundRecordsErrorType(t *testing.T) {
	rec := &chain.Recorder{}
	rec.AddRound(chain.Round{
		Start:     time.Now(),
		Status:    chain.ClassifyStatus(chain.ErrBulkheadFull),
		ErrorType: chain.ClassifyErrorType(chain.ErrBulkheadFull),
		Err:       chain.ErrBulkheadFull,
	})

	a, ok := rec.LastRound()
	assert.That(t, ok).True("the refusal is a round")
	assert.That(t, a.Status).Equal("error")
	assert.That(t, a.ErrorType).Equal("bulkhead_full")
	assert.That(t, errors.Is(a.Err, chain.ErrBulkheadFull)).True("the rejection rides on the round")
}

// Reset clears everything so one Recorder can serve another call — the supported
// way to pool them. Timeout is the only call-level fact it carries, and the
// rounds are the record.
func TestResetClearsTheRecord(t *testing.T) {
	rec := &chain.Recorder{}
	rec.Timeout = 50 * time.Millisecond
	rec.SetFault(&chain.Injection{Kind: "timeout"})
	rec.AddRound(chain.Round{Start: time.Now(), Duration: time.Millisecond, Status: "ok"})

	rec.Reset()

	assert.That(t, rec.Timeout).Equal(time.Duration(0))
	assert.That(t, rec.Fault == nil).True("the fire is cleared with the rest")
	assert.That(t, len(rec.Rounds())).Equal(0)
}

// The two axes are independent: Status is the call's result, ErrorType names what
// the chain did about it.
func TestClassifyAxes(t *testing.T) {
	assert.That(t, chain.ClassifyStatus(nil)).Equal("ok")
	assert.That(t, chain.ClassifyStatus(chain.ErrRateLimited)).Equal("error")

	assert.That(t, chain.ClassifyErrorType(chain.ErrRateLimited)).Equal("rate_limited")
	assert.That(t, chain.ClassifyErrorType(chain.ErrCircuitOpen)).Equal("circuit_open")
	assert.That(t, chain.ClassifyErrorType(chain.ErrBulkheadFull)).Equal("bulkhead_full")
	assert.That(t, chain.ClassifyErrorType(chain.ErrRetryBudgetExceeded)).Equal("retry_budget_exceeded")
	assert.That(t, chain.ClassifyErrorType(context.DeadlineExceeded)).Equal("timeout")
	assert.That(t, chain.ClassifyErrorType(errors.New("downstream"))).Equal("")
}

// chanErr keeps the test's second round's error simple and typed.
func chanErr() error { return errors.New("downstream refused") }
