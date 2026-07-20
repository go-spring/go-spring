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

package aspect_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go-spring.org/spring/aspect"
	"go-spring.org/stdlib/testing/assert"
)

// order records the entry/exit sequence of interceptors so tests can assert the
// composition order.
func recordingInterceptor(log *[]string, name string) aspect.Interceptor {
	return aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		*log = append(*log, "enter "+name)
		v, err := jp.Proceed(jp.Context)
		*log = append(*log, "exit "+name)
		return v, err
	})
}

func TestChainPassThrough(t *testing.T) {
	// A nil chain and an empty chain both run the target exactly once.
	for _, c := range []*aspect.Chain{nil, aspect.NewChain()} {
		calls := 0
		v, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
			calls++
			return 42, nil
		})
		assert.Error(t, err).Nil()
		assert.That(t, v).Equal(42)
		assert.That(t, calls).Equal(1)
	}
}

func TestChainOrderOutermostFirst(t *testing.T) {
	var log []string
	c := aspect.NewChain(
		recordingInterceptor(&log, "a"),
		recordingInterceptor(&log, "b"),
	)
	_, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
		log = append(log, "target")
		return nil, nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, log).Equal([]string{
		"enter a", "enter b", "target", "exit b", "exit a",
	})
}

func TestChainNilInterceptorsIgnored(t *testing.T) {
	var log []string
	c := aspect.NewChain(nil, recordingInterceptor(&log, "a"), nil)
	err := c.RunE(context.Background(), "m", func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
	assert.That(t, log).Equal([]string{"enter a", "exit a"})
}

func TestChainWithIsImmutable(t *testing.T) {
	var log []string
	base := aspect.NewChain(recordingInterceptor(&log, "base"))
	extended := base.With(recordingInterceptor(&log, "extra"))

	log = nil
	_ = base.RunE(context.Background(), "m", func(context.Context) error { return nil })
	assert.That(t, log).Equal([]string{"enter base", "exit base"})

	log = nil
	_ = extended.RunE(context.Background(), "m", func(context.Context) error { return nil })
	assert.That(t, log).Equal([]string{"enter base", "enter extra", "exit extra", "exit base"})
}

func TestRunEPropagatesError(t *testing.T) {
	sentinel := errors.New("boom")
	err := aspect.NewChain().RunE(context.Background(), "m", func(context.Context) error {
		return sentinel
	})
	assert.Error(t, err).Is(sentinel)
}

func TestAroundTypeSafe(t *testing.T) {
	c := aspect.NewChain(aspect.Timing(func(string, time.Duration, error) {}))
	got, err := aspect.Around(c, context.Background(), "m",
		func(context.Context) (string, error) { return "hello", nil })
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal("hello")
}

func TestAroundShortCircuitWrongTypeYieldsZero(t *testing.T) {
	// An interceptor short-circuits with an int; Around[string] must not panic
	// and returns the zero value of the requested type.
	c := aspect.NewChain(aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		return 123, nil
	}))
	got, err := aspect.Around(c, context.Background(), "m",
		func(context.Context) (string, error) { return "unused", nil })
	assert.Error(t, err).Nil()
	assert.That(t, got).Equal("")
}

func TestRecoverTurnsPanicIntoError(t *testing.T) {
	c := aspect.NewChain(aspect.Recover())
	_, err := c.Run(context.Background(), "risky", func(context.Context) (any, error) {
		panic("kaboom")
	})
	assert.Error(t, err).NotNil()
	assert.Error(t, err).Matches("recovered panic in \"risky\"")
}

func TestRecoverPreservesErrorPanic(t *testing.T) {
	sentinel := errors.New("typed panic")
	c := aspect.NewChain(aspect.Recover())
	_, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
		panic(sentinel)
	})
	assert.Error(t, err).Is(sentinel)
}

func TestTimingReportsOnSuccessAndFailure(t *testing.T) {
	var methods []string
	var errs []error
	report := func(method string, _ time.Duration, err error) {
		methods = append(methods, method)
		errs = append(errs, err)
	}
	c := aspect.NewChain(aspect.Timing(report))

	_ = c.RunE(context.Background(), "ok", func(context.Context) error { return nil })
	boom := errors.New("boom")
	_ = c.RunE(context.Background(), "bad", func(context.Context) error { return boom })

	assert.That(t, methods).Equal([]string{"ok", "bad"})
	assert.Error(t, errs[0]).Nil()
	assert.Error(t, errs[1]).Is(boom)
}

func TestCacheHitSkipsTarget(t *testing.T) {
	store := &aspect.MemoryStore{}
	key := func(jp *aspect.Joinpoint) string { return jp.Method }
	c := aspect.NewChain(aspect.Cache(store, key, time.Minute))

	calls := 0
	target := func(context.Context) (any, error) {
		calls++
		return "value", nil
	}
	v1, _ := c.Run(context.Background(), "k", target)
	v2, _ := c.Run(context.Background(), "k", target)
	assert.That(t, v1).Equal("value")
	assert.That(t, v2).Equal("value")
	assert.That(t, calls).Equal(1) // second call served from cache
}

func TestCacheDoesNotStoreOnError(t *testing.T) {
	store := &aspect.MemoryStore{}
	key := func(jp *aspect.Joinpoint) string { return jp.Method }
	c := aspect.NewChain(aspect.Cache(store, key, time.Minute))

	calls := 0
	target := func(context.Context) (any, error) {
		calls++
		return nil, errors.New("fail")
	}
	_, _ = c.Run(context.Background(), "k", target)
	_, _ = c.Run(context.Background(), "k", target)
	assert.That(t, calls).Equal(2) // errors are not cached
}

func TestCacheEmptyKeyBypasses(t *testing.T) {
	store := &aspect.MemoryStore{}
	key := func(*aspect.Joinpoint) string { return "" }
	c := aspect.NewChain(aspect.Cache(store, key, time.Minute))
	calls := 0
	target := func(context.Context) (any, error) { calls++; return 1, nil }
	_, _ = c.Run(context.Background(), "k", target)
	_, _ = c.Run(context.Background(), "k", target)
	assert.That(t, calls).Equal(2)
}

func TestMemoryStoreExpiry(t *testing.T) {
	store := &aspect.MemoryStore{}
	store.Set("k", "v", 20*time.Millisecond)
	v, ok := store.Get("k")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("v")

	time.Sleep(40 * time.Millisecond)
	_, ok = store.Get("k")
	assert.That(t, ok).False()
}

func TestMemoryStoreNoExpiry(t *testing.T) {
	store := &aspect.MemoryStore{}
	store.Set("k", "v", 0)
	v, ok := store.Get("k")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("v")
}

// fakeTx is an in-memory TxManager used to exercise the Transactional
// interceptor without any real database dependency.
type fakeTx struct {
	beginErr    error
	commitErr   error
	rollbackErr error
	committed   bool
	rolledBack  bool
}

type txKey struct{}

func (f *fakeTx) Begin(ctx context.Context) (context.Context, any, error) {
	if f.beginErr != nil {
		return ctx, nil, f.beginErr
	}
	return context.WithValue(ctx, txKey{}, f), f, nil
}

func (f *fakeTx) Commit(any) error   { f.committed = true; return f.commitErr }
func (f *fakeTx) Rollback(any) error { f.rolledBack = true; return f.rollbackErr }

func TestTransactionalCommitsOnSuccess(t *testing.T) {
	tm := &fakeTx{}
	c := aspect.NewChain(aspect.Transactional(tm))
	var sawTx bool
	_, err := c.Run(context.Background(), "m", func(ctx context.Context) (any, error) {
		_, sawTx = ctx.Value(txKey{}).(*fakeTx) // business code runs inside the tx
		return nil, nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, sawTx).True()
	assert.That(t, tm.committed).True()
	assert.That(t, tm.rolledBack).False()
}

func TestTransactionalRollsBackOnError(t *testing.T) {
	tm := &fakeTx{}
	boom := errors.New("boom")
	c := aspect.NewChain(aspect.Transactional(tm))
	_, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
		return nil, boom
	})
	assert.Error(t, err).Is(boom)
	assert.That(t, tm.committed).False()
	assert.That(t, tm.rolledBack).True()
}

func TestTransactionalRollsBackOnPanic(t *testing.T) {
	tm := &fakeTx{}
	// Recover sits outside Transactional so the panic rolls back then becomes an error.
	c := aspect.NewChain(aspect.Recover(), aspect.Transactional(tm))
	_, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
		panic("kaboom")
	})
	assert.Error(t, err).NotNil()
	assert.That(t, tm.rolledBack).True()
	assert.That(t, tm.committed).False()
}

func TestTransactionalBeginError(t *testing.T) {
	tm := &fakeTx{beginErr: errors.New("no conn")}
	c := aspect.NewChain(aspect.Transactional(tm))
	calls := 0
	_, err := c.Run(context.Background(), "m", func(context.Context) (any, error) {
		calls++
		return nil, nil
	})
	assert.Error(t, err).NotNil()
	assert.That(t, calls).Equal(0) // target never ran
}

func TestOnlyAppliesToListedMethods(t *testing.T) {
	var applied []string
	inner := aspect.InterceptorFunc(func(jp *aspect.Joinpoint) (any, error) {
		applied = append(applied, jp.Method)
		return jp.Proceed(jp.Context)
	})
	c := aspect.NewChain(aspect.Only(inner, "guarded"))

	_ = c.RunE(context.Background(), "guarded", func(context.Context) error { return nil })
	_ = c.RunE(context.Background(), "open", func(context.Context) error { return nil })
	assert.That(t, applied).Equal([]string{"guarded"})
}

func TestNewHandlerNilChainReturnsNext(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	got := aspect.NewHandler(next, nil, nil)
	assert.That(t, fmt.Sprintf("%p", got)).Equal(fmt.Sprintf("%p", next))
}

func TestNewHandlerServesOnceAndReportsStatus(t *testing.T) {
	var reported []error
	var serves int
	chain := aspect.NewChain(aspect.Timing(func(_ string, _ time.Duration, err error) {
		reported = append(reported, err)
	}))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serves++
		w.WriteHeader(http.StatusInternalServerError)
	})
	h := aspect.NewHandler(next, chain, func(r *http.Request) string { return r.URL.Path })

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.That(t, serves).Equal(1)
	assert.That(t, rec.Code).Equal(http.StatusInternalServerError)
	assert.That(t, len(reported)).Equal(1)
	assert.Error(t, reported[0]).NotNil() // 5xx surfaced as error
}

func TestNewHandlerRecoverIntegration(t *testing.T) {
	chain := aspect.NewChain(aspect.Recover())
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	h := aspect.NewHandler(next, chain, nil)
	rec := httptest.NewRecorder()
	// Recover swallows the panic into an error the handler discards; no re-panic.
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
}

func TestConcurrentChainUse(t *testing.T) {
	store := &aspect.MemoryStore{}
	c := aspect.NewChain(
		aspect.Timing(func(string, time.Duration, error) {}),
		aspect.Cache(store, func(jp *aspect.Joinpoint) string { return jp.Method }, time.Minute),
	)
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			_, _ = c.Run(context.Background(), "k", func(context.Context) (any, error) {
				return "v", nil
			})
		})
	}
	wg.Wait()
	v, ok := store.Get("k")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("v")
}

// --- Examples: the decorator + DI convention (acceptance scenario) ---

// OrderService is the business interface consumers depend on. They never see the
// aspect decorator that wraps it.
type OrderService interface {
	Place(ctx context.Context, id string) (string, error)
}

type orderService struct{}

func (s *orderService) Place(_ context.Context, id string) (string, error) {
	return "receipt:" + id, nil
}

// orderServiceTx decorates OrderService with a declarative transaction, sharing
// the same interface so callers are unaware.
type orderServiceTx struct {
	inner OrderService
	chain *aspect.Chain
}

func (a *orderServiceTx) Place(ctx context.Context, id string) (string, error) {
	return aspect.Around(a.chain, ctx, "OrderService.Place",
		func(ctx context.Context) (string, error) { return a.inner.Place(ctx, id) })
}

// exampleTx is a demonstration TxManager that prints its lifecycle.
type exampleTx struct{}

func (exampleTx) Begin(ctx context.Context) (context.Context, any, error) {
	fmt.Println("BEGIN")
	return ctx, struct{}{}, nil
}
func (exampleTx) Commit(any) error   { fmt.Println("COMMIT"); return nil }
func (exampleTx) Rollback(any) error { fmt.Println("ROLLBACK"); return nil }

func Example_transactional() {
	chain := aspect.NewChain(aspect.Transactional(exampleTx{}))
	var svc OrderService = &orderServiceTx{inner: &orderService{}, chain: chain}

	receipt, err := svc.Place(context.Background(), "42")
	fmt.Println(receipt, err)
	// Output:
	// BEGIN
	// COMMIT
	// receipt:42 <nil>
}

func Example_cache() {
	store := &aspect.MemoryStore{}
	chain := aspect.NewChain(aspect.Cache(store,
		func(jp *aspect.Joinpoint) string { return jp.Method }, time.Minute))

	calls := 0
	load := func(ctx context.Context) (string, error) {
		calls++
		return "loaded", nil
	}
	for range 3 {
		v, _ := aspect.Around(chain, context.Background(), "config", load)
		fmt.Println(v)
	}
	fmt.Println("db calls:", calls)
	// Output:
	// loaded
	// loaded
	// loaded
	// db calls: 1
}
