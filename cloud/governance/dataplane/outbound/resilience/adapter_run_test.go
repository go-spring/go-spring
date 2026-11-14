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
	"fmt"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

var errCacheMiss = errors.New("cache miss")

// stubExecutor is a chain.Executor whose Execute does not call fn: it is how a
// test reproduces "the attempt was short-circuited before the operation ran"
// (a fault injector, for example).
type stubExecutor struct {
	err error
}

func (s *stubExecutor) Execute(context.Context, chain.Operation, *chain.Recorder, chain.RoundFunc) error {
	return s.err
}

func (s *stubExecutor) Close() error { return nil }

func TestRunNilExecutorIsPassThrough(t *testing.T) {
	var calls int
	got, err := Run(context.Background(), nil, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (int, error) {
		calls++
		return 7, nil
	})
	assert.Error(t, err).Nil()
	assert.Number(t, got).Equal(7)
	assert.Number(t, calls).Equal(1)
}

func TestRunReturnsResultAndFailure(t *testing.T) {
	boom := transient{}
	var calls int

	// Success: the operation's value comes back.
	got, err := Run(context.Background(), newBuiltin(t, TimeoutRetry{}), chain.Operation{Service: "svc"},
		func(context.Context, chain.Peer) (string, error) { return "reply", nil })
	assert.Error(t, err).Nil()
	assert.String(t, got).Equal("reply")

	// Failure: fn's own error is returned (and the retry loop ran).
	e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 1}})
	_, err = Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (string, error) {
		calls++
		return "", boom
	})
	assert.Error(t, err).Is(boom)
	assert.Number(t, calls).Equal(2) // one attempt plus one retry
}

// A tolerated sentinel is a normal outcome: it is returned verbatim but does not
// count against the breaker and does not trigger a retry.
func TestRunToleratedErrorIsNotAFault(t *testing.T) {
	t.Run("no retry", func(t *testing.T) {
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 3}})
		var calls int
		_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
			calls++
			return struct{}{}, errCacheMiss
		}, Tolerate(errCacheMiss))
		assert.Error(t, err).Is(errCacheMiss)
		assert.Number(t, calls).Equal(1)
	})

	t.Run("does not trip the breaker", func(t *testing.T) {
		e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})
		call := func() error {
			_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
				return struct{}{}, errCacheMiss
			}, Tolerate(errCacheMiss))
			return err
		}
		assert.Error(t, call()).Is(errCacheMiss)
		// The breaker is still closed: if the miss had counted, this second call
		// would be short-circuited with ErrCircuitOpen.
		assert.Error(t, call()).Is(errCacheMiss)
	})

	t.Run("stacks", func(t *testing.T) {
		other := errors.New("no rows")
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 3}})
		_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
			return struct{}{}, other
		}, Tolerate(errCacheMiss), Tolerate(other))
		assert.Error(t, err).Is(other)
	})

	t.Run("class predicate", func(t *testing.T) {
		business := &typedErr{code: 550, normal: true}
		e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})
		call := func() error {
			_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
				return struct{}{}, business
			}, TolerateFunc(func(err error) bool {
				var te *typedErr
				return errors.As(err, &te) && te.normal
			}))
			return err
		}
		assert.Error(t, call()).Is(business)
		// If the class had counted as a fault, this second call would be
		// short-circuited with ErrCircuitOpen.
		assert.Error(t, call()).Is(business)
	})
}

// typedErr is a stand-in for a client library's error family: one type carrying
// a class flag, which no single sentinel can name.
type typedErr struct {
	code   int
	normal bool
}

func (e *typedErr) Error() string { return fmt.Sprintf("code %d", e.code) }

// RetryOn declares a class as retry-eligible without changing what the caller
// receives.
func TestRunRetryOn(t *testing.T) {
	t.Run("retries the declared class", func(t *testing.T) {
		boom := errors.New("transient")
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}})
		var calls int
		_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
			calls++
			return struct{}{}, boom
		}, RetryOn(func(err error) bool { return errors.Is(err, boom) }))
		assert.Error(t, err).Is(boom) // the caller gets the original, not the wrapper
		assert.Number(t, calls).Equal(3)
	})

	t.Run("unmatched error runs once", func(t *testing.T) {
		boom := errors.New("transient")
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}})
		var calls int
		_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
			calls++
			return struct{}{}, boom
		}, RetryOn(func(error) bool { return false }))
		assert.Error(t, err).Is(boom)
		assert.Number(t, calls).Equal(1)
	})

	t.Run("tolerated wins over retry", func(t *testing.T) {
		e := newBuiltin(t, TimeoutRetry{Retry: Retry{MaxRetries: 2}})
		var calls int
		_, err := Run(context.Background(), e, chain.Operation{Service: "svc"}, func(context.Context, chain.Peer) (struct{}, error) {
			calls++
			return struct{}{}, errCacheMiss
		}, Tolerate(errCacheMiss), RetryOn(func(error) bool { return true }))
		assert.Error(t, err).Is(errCacheMiss)
		assert.Number(t, calls).Equal(1)
	})
}

func TestRunRejectionIsReturnedVerbatim(t *testing.T) {
	e := newBuiltin(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1})
	ctx := context.Background()
	op := chain.Operation{Service: "svc"}

	// Spend the budget.
	_, err := Run(ctx, e, op, func(context.Context, chain.Peer) (struct{}, error) { return struct{}{}, nil })
	assert.Error(t, err).Nil()

	var ran bool
	_, err = Run(ctx, e, op, func(context.Context, chain.Peer) (struct{}, error) {
		ran = true
		return struct{}{}, nil
	})
	assert.Error(t, err).Is(chain.ErrRateLimited)
	assert.That(t, ran).False()
}

// When the attempt never reached fn, exec's error must be preferred: callErr is
// nil, and returning it would swallow the injected failure as success.
func TestRunPrefersExecutorErrorWhenFnNeverRan(t *testing.T) {
	injected := errors.New("injected")
	_, err := Run(context.Background(), &stubExecutor{err: injected}, chain.Operation{Service: "svc"},
		func(context.Context, chain.Peer) (int, error) { return 42, nil })
	assert.Error(t, err).Is(injected)
}

func TestIsRejection(t *testing.T) {
	for _, sentinel := range []error{
		chain.ErrRateLimited, chain.ErrCircuitOpen, chain.ErrBulkheadFull, chain.ErrRetryExceeded,
	} {
		assert.That(t, IsRejection(sentinel)).True()
		assert.That(t, IsRejection(fmt.Errorf("wrapped: %w", sentinel))).True()
	}
	assert.That(t, IsRejection(nil)).False()
	assert.That(t, IsRejection(errors.New("ordinary"))).False()
}
