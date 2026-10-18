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

package resilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// TestEveryGovernedCallRecordsAtLeastOneRound pins the invariant the record's
// readers rely on: a call that ran the chain and was handed a recorder always
// leaves at least one round — a served attempt, or the gate that refused it.
// "No rounds" therefore means "no record was taken" (nil recorder, or a call
// that never entered the chain), never "the call was rejected"; a refusal is a
// decision the chain made, so it is recorded like everything else.
func TestEveryGovernedCallRecordsAtLeastOneRound(t *testing.T) {
	t.Run("served", func(t *testing.T) {
		rec := &chain.Recorder{}
		err := newBuiltin(t, Policy{}).Execute(context.Background(), chain.Operation{}, rec,
			func(context.Context) error { return nil })
		assert.Error(t, err).Nil()
		assert.Number(t, len(rec.Rounds())).Equal(1)
		assert.String(t, rec.Rounds()[0].Status).Equal("ok")
	})

	t.Run("refused-before-the-downstream", func(t *testing.T) {
		e := newBuiltin(t, Policy{RateLimit: 1, Burst: 1})
		// Spend the burst so the next call is refused.
		assert.Error(t, e.Execute(context.Background(), chain.Operation{}, nil,
			func(context.Context) error { return nil })).Nil()

		rec := &chain.Recorder{}
		err := e.Execute(context.Background(), chain.Operation{}, rec,
			func(context.Context) error { return nil })
		assert.Error(t, err).Is(chain.ErrRateLimited)
		assert.Number(t, len(rec.Rounds())).Equal(1)
		assert.String(t, rec.Rounds()[0].ErrorType).Equal("rate_limited")
	})

	t.Run("every-retry-is-a-round", func(t *testing.T) {
		e := newBuiltin(t, Policy{Retry: chain.RetryPolicy{MaxRetries: 2, InitialInterval: time.Millisecond}})
		rec := &chain.Recorder{}
		err := e.Execute(context.Background(), chain.Operation{}, rec,
			func(context.Context) error { return errors.New("boom") })
		assert.Error(t, err).NotNil()
		assert.Number(t, len(rec.Rounds())).Equal(3)
	})

	t.Run("a-declined-record-is-not-an-empty-record", func(t *testing.T) {
		// nil means "record nothing": the call runs, and there is no record to
		// read — which is why a reader must never conclude anything from an
		// absent recorder it never asked for.
		err := newBuiltin(t, Policy{}).Execute(context.Background(), chain.Operation{}, nil,
			func(context.Context) error { return nil })
		assert.Error(t, err).Nil()
	})
}
