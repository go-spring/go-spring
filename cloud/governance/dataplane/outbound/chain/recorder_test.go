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
	"fmt"
	"os"
	"testing"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
)

// The produced observability block is the write-back counterpart of the declared
// one: three groups, appended the same way, read by the emitter. The writes land
// on the LAST round — the one the loop created for the running attempt.
func TestProducedObservabilityIsKeptPerRound(t *testing.T) {
	rec := &chain.Recorder{}
	rd := rec.NewRound()
	rec.LastRound().AddLogFields(log.String("cache", "hit"))
	rec.LastRound().AddSpanAttrs(attribute.String("cache.result", "hit"))
	rec.LastRound().AddMetricAttrs(attribute.String("cache.result", "hit"))

	assert.Number(t, len(rd.Observed.LogFields)).Equal(1)
	assert.Number(t, len(rd.Observed.SpanAttrs)).Equal(1)
	assert.Number(t, len(rd.Observed.MetricAttrs)).Equal(1)
	assert.String(t, rd.Observed.SpanAttrs[0].Value.AsString()).Equal("hit")
}

// The fire is a PER-ROUND event: the fault layer sets it on the round its
// attempt ran in, and every fired round keeps its own — a clean retry after a
// fired attempt does not erase that attempt's fire.
func TestFaultIsPerRound(t *testing.T) {
	rec := &chain.Recorder{}

	first := rec.NewRound()
	first.Fault = &kfault.Result{Name: "redis:cache[0]", Inject: true, Error: "reset"}
	assert.String(t, first.Fault.Error).Equal("reset")
	assert.String(t, first.Fault.Name).Equal("redis:cache[0]")

	second := rec.NewRound()
	second.Fault = &kfault.Result{Name: "fire", Inject: true, Error: "timeout"}
	third := rec.NewRound() // the clean retry that ended the call
	assert.That(t, third.Fault == nil).True()

	// The earlier fired rounds keep their events; the call-level view is the
	// rounds themselves.
	assert.String(t, rec.Rounds[0].Fault.Error).Equal("reset")
	assert.String(t, rec.Rounds[1].Fault.Error).Equal("timeout")
}

// Each round is opened and filled in place, and the rounds stay independent of
// each other: every field belongs to the round that measured it.
func TestNewRoundAppendsTheRounds(t *testing.T) {
	rec := &chain.Recorder{}
	start := time.Now()

	rd := rec.NewRound()
	rd.Start, rd.Duration, rd.Status = start, 7*time.Millisecond, "ok"
	rd = rec.NewRound()
	rd.Start, rd.Duration, rd.Status = time.Now(), time.Millisecond, "error"
	rd.ErrorType, rd.Error = "circuit_open", chanErr()

	rounds := rec.Rounds
	assert.Number(t, len(rounds)).Equal(2)
	assert.That(t, rounds[0].Duration).Equal(7 * time.Millisecond)
	assert.That(t, rounds[0].Status).Equal("ok")
	assert.String(t, string(rounds[1].ErrorType)).Equal("circuit_open")

	last := rec.LastRound()
	assert.That(t, last != nil).True("the last round is the one just appended")
	assert.That(t, errors.Is(last.Error, chanErr())).False("each round carries its own error") // same text, different value
	assert.String(t, last.Error.Error()).Equal("downstream refused")
}

// A round the chain refused before touching the downstream is still a round: it
// carries the gate's verdict, which is how "nothing ran" is told apart from "ran
// and failed".
func TestRefusedRoundRecordsErrorType(t *testing.T) {
	rec := &chain.Recorder{}
	rd := rec.NewRound()
	rd.Start = time.Now()
	rd.Status = chain.ClassifyStatus(chain.ErrBulkheadFull)
	rd.ErrorType = chain.RoundErrorType(chain.ErrBulkheadFull)
	rd.Error = chain.ErrBulkheadFull

	a := rec.LastRound()
	assert.That(t, a != nil).True("the refusal is a round")
	assert.That(t, a.Status).Equal("error")
	assert.String(t, string(a.ErrorType)).Equal("bulkhead_full")
	assert.That(t, errors.Is(a.Error, chain.ErrBulkheadFull)).True("the rejection rides on the round")
}

// The two axes are independent: Status is the call's result, ErrorType names what
// the chain did about it.
func TestClassifyAxes(t *testing.T) {
	assert.That(t, chain.ClassifyStatus(nil)).Equal("ok")
	assert.That(t, chain.ClassifyStatus(chain.ErrRateLimited)).Equal("error")

	assert.String(t, string(chain.RoundErrorType(chain.ErrRateLimited))).Equal("rate_limited")
	assert.String(t, string(chain.RoundErrorType(chain.ErrCircuitOpen))).Equal("circuit_open")
	assert.String(t, string(chain.RoundErrorType(chain.ErrBulkheadFull))).Equal("bulkhead_full")
	assert.String(t, string(chain.RoundErrorType(chain.ErrRetryExceeded))).Equal("retry_exceeded")
	assert.String(t, string(chain.RoundErrorType(context.DeadlineExceeded))).Equal("unknown")
	assert.String(t, string(chain.RoundErrorType(os.ErrDeadlineExceeded))).Equal("io_timeout")
	assert.String(t, string(chain.RoundErrorType(fmt.Errorf("read: %w", os.ErrDeadlineExceeded)))).Equal("io_timeout")
	// context.DeadlineExceeded implements net.Error with Timeout()==true; the
	// context signal must not read as a transport deadline.
	assert.That(t, chain.IsIOTimeout(context.DeadlineExceeded)).False()
	assert.That(t, chain.IsIOTimeout(os.ErrDeadlineExceeded)).True()
	assert.String(t, string(chain.RoundErrorType(fmt.Errorf("%w: %w", chain.ErrCallTimeout, context.DeadlineExceeded)))).Equal("call_timeout")
	assert.String(t, string(chain.RoundErrorType(errors.New("boom")))).Equal("unknown")
	assert.String(t, string(chain.RoundErrorType(chain.ConnectError(errors.New("refused"))))).Equal("connect_failed")
	assert.String(t, string(chain.RoundErrorType(chain.ResourceError(errors.New("pool exhausted"))))).Equal("resource_exhausted")
	assert.String(t, string(chain.RoundErrorType(nil))).Equal("")
	assert.String(t, string(chain.RoundErrorType(context.Canceled))).Equal("context_canceled")
}

// CallErrorType is the CALL-level counterpart and is total: a failed call is
// always attributable, and a caller abort is not read as a downstream failure.
func TestCallErrorType(t *testing.T) {
	assert.String(t, string(chain.CallErrorType(context.Background(), nil))).Equal("")
	assert.String(t, string(chain.CallErrorType(context.Background(), chain.ErrCircuitOpen))).Equal("circuit_open")
	assert.String(t, string(chain.CallErrorType(context.Background(), context.DeadlineExceeded))).Equal("unknown")
	assert.String(t, string(chain.CallErrorType(context.Background(), fmt.Errorf("%w: %w", chain.ErrCallTimeout, context.DeadlineExceeded)))).Equal("call_timeout")
	assert.That(t, chain.CallErrorType(context.Background(), errors.New("refused"))).Equal(chain.ErrorTypeUnknown)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	assert.That(t, chain.CallErrorType(cancelled, context.Canceled)).Equal(chain.ErrorTypeContextCanceled)

	// A deadline the CALLER set is an abandoned call, not the chain's budget.
	expired, cancelDeadline := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancelDeadline()
	assert.That(t, chain.CallErrorType(expired, context.DeadlineExceeded)).Equal(chain.ErrorTypeContextCanceled)
}

// chanErr keeps the test's second round's error simple and typed.
func chanErr() error { return errors.New("downstream refused") }
