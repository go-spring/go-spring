/*
 * Copyright 2024 The Go-Spring Authors.
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

package goutil_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/goutil"
	"github.com/go-spring/stdlib/testing/assert"
)

func TestGo(t *testing.T) {
	t.Run("panic recovery", func(t *testing.T) {
		var s string
		goutil.Go(t.Context(), func(ctx context.Context) {
			panic("something is wrong")
		}, goutil.InheritCancel).Wait()
		assert.That(t, s).Equal("")
	})

	t.Run("normal execution", func(t *testing.T) {
		var s string
		goutil.Go(t.Context(), func(ctx context.Context) {
			s = "hello world!"
		}, goutil.InheritCancel).Wait()
		assert.That(t, s).Equal("hello world!")
	})

	t.Run("context cancel with withoutCancel=false", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		resultCh := make(chan string, 1)
		status := goutil.Go(ctx, func(ctx context.Context) {
			<-ctx.Done()
			select {
			case <-ctx.Done():
				resultCh <- "context was cancelled (expected with withoutCancel=false)"
			default:
				resultCh <- "context was not cancelled"
			}
		}, goutil.InheritCancel)

		time.Sleep(5 * time.Millisecond)
		cancel()
		status.Wait()

		result := <-resultCh
		assert.That(t, result).Equal("context was cancelled (expected with withoutCancel=false)")
	})

	t.Run("context cancel with withoutCancel=true", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		resultCh := make(chan string, 1)
		cancel()

		goutil.Go(ctx, func(ctx context.Context) {
			select {
			case <-time.After(50 * time.Millisecond):
				resultCh <- "context was not cancelled (as expected with withoutCancel=true)"
			case <-ctx.Done():
				resultCh <- "context was cancelled (unexpected with withoutCancel=true)"
			}
		}, goutil.DetachCancel).Wait()

		result := <-resultCh
		assert.That(t, result).Equal("context was not cancelled (as expected with withoutCancel=true)")
	})
}

func TestGoValue(t *testing.T) {
	t.Run("panic recovery", func(t *testing.T) {
		s, err := goutil.GoValue(t.Context(), func(ctx context.Context) (string, error) {
			panic("something is wrong")
		}, goutil.InheritCancel).Wait()
		assert.That(t, s).Equal("")
		assert.Error(t, err).Matches("panic recovered: .*")
	})

	t.Run("successful execution with int", func(t *testing.T) {
		i, err := goutil.GoValue(t.Context(), func(ctx context.Context) (int, error) {
			return 42, nil
		}, goutil.InheritCancel).Wait()
		assert.That(t, err).Nil()
		assert.That(t, i).Equal(42)
	})

	t.Run("successful execution with string", func(t *testing.T) {
		s, err := goutil.GoValue(t.Context(), func(ctx context.Context) (string, error) {
			return "hello world!", nil
		}, goutil.InheritCancel).Wait()
		assert.That(t, err).Nil()
		assert.That(t, s).Equal("hello world!")
	})

	t.Run("multiple safegos", func(t *testing.T) {
		var arr []*goutil.ValueStatus[int]
		for i := range 3 {
			arr = append(arr, goutil.GoValue(t.Context(), func(ctx context.Context) (int, error) {
				return i, nil
			}, goutil.InheritCancel))
		}
		for i, g := range arr {
			v, err := g.Wait()
			assert.That(t, v).Equal(i)
			assert.That(t, err).Nil()
		}
	})

	t.Run("error return", func(t *testing.T) {
		expectedErr := errutil.Explain(nil, "expected error")
		_, err := goutil.GoValue(t.Context(), func(ctx context.Context) (string, error) {
			return "", expectedErr
		}, goutil.InheritCancel).Wait()
		assert.That(t, err).Equal(expectedErr)
	})

	t.Run("context cancel with withoutCancel=false", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		resultFuture := goutil.GoValue(ctx, func(ctx context.Context) (string, error) {
			<-ctx.Done()
			select {
			case <-ctx.Done():
				return "context was cancelled as expected", nil
			default:
				return "context was not cancelled", nil
			}
		}, goutil.InheritCancel)

		time.Sleep(5 * time.Millisecond)
		cancel()

		result, err := resultFuture.Wait()
		assert.That(t, err).Nil()
		assert.That(t, result).Equal("context was cancelled as expected")
	})

	t.Run("context cancel with withoutCancel=true", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		result, err := goutil.GoValue(ctx, func(ctx context.Context) (string, error) {
			select {
			case <-time.After(50 * time.Millisecond):
				return "context was not cancelled (as expected with withoutCancel=true)", nil
			case <-ctx.Done():
				return "context was cancelled (unexpected with withoutCancel=true)", nil
			}
		}, goutil.DetachCancel).Wait()

		assert.That(t, err).Nil()
		assert.That(t, result).Equal("context was not cancelled (as expected with withoutCancel=true)")
	})

	t.Run("context value inheritance with withoutCancel", func(t *testing.T) {
		key := "test_key"
		value := "test_value"
		// nolint: staticcheck
		ctx := context.WithValue(context.Background(), key, value)
		ctx, cancel := context.WithCancel(ctx)
		cancel()

		result, err := goutil.GoValue(ctx, func(ctx context.Context) (string, error) {
			retrievedValue := ctx.Value(key)
			if retrievedValue == nil {
				return "value not found", nil
			}
			if retrievedValueStr, ok := retrievedValue.(string); ok {
				return "value: " + retrievedValueStr, nil
			}
			return "value not string", nil
		}, goutil.DetachCancel).Wait()

		assert.That(t, err).Nil()
		assert.That(t, result).Equal("value: test_value")
	})
}
