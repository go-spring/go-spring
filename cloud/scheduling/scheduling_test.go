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

package scheduling_test

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/scheduling"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
)

func TestConcurrencyPolicyString(t *testing.T) {
	assert.That(t, scheduling.Skip.String()).Equal("skip")
	assert.That(t, scheduling.Queue.String()).Equal("queue")
	assert.That(t, scheduling.Replace.String()).Equal("replace")
	assert.That(t, scheduling.ConcurrencyPolicy(99).String()).Equal("unknown")
}

// TestRunCarriesItsOwnTraceID pins the run as the trace identity's scope: with
// no tracer to give the run a span of its own, the context it receives carries
// a trace_id, and two runs never share one — the id is minted per run, so it
// dies with the run's context instead of covering everything the task's loop
// does.
func TestRunCarriesItsOwnTraceID(t *testing.T) {
	withoutTracer(t)

	s := scheduling.NewScheduler()

	var mu sync.Mutex
	var runs []context.Context
	_, err := s.Schedule(mustJob(t, "traced", scheduling.FixedRate(20*time.Millisecond),
		func(ctx context.Context) error {
			mu.Lock()
			runs = append(runs, ctx)
			mu.Unlock()
			return nil
		}))
	assert.Error(t, err).Nil()

	assert.Error(t, s.Start(context.Background())).Nil()
	time.Sleep(110 * time.Millisecond) // ~5 runs
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	assert.Error(t, s.Stop(stopCtx)).Nil()

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(runs) >= 2).True("expected at least 2 runs")

	first, second := renderFields(log.CarriedFields(runs[0])), renderFields(log.CarriedFields(runs[1]))
	assert.String(t, first).Contains("trace_id=")
	assert.That(t, first != second).True("two runs must not share a trace_id")
}

// renderFields renders fields the way a log line does, so a test can read a
// field's value through the public surface instead of the Field internals.
func renderFields(fields []log.Field) string {
	var buf bytes.Buffer
	log.EncodeFields(log.NewTextEncoder(&buf, " "), fields)
	return buf.String()
}
