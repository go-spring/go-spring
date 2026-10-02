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

package singleton

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

type resource struct{ id int }

func TestInitRunsConstructorOnce(t *testing.T) {
	var s Singleton[*resource]
	calls := 0
	newFn := func() (*resource, error) {
		calls++
		return &resource{id: 1}, nil
	}

	v, err := s.Init(newFn)
	assert.Error(t, err).Nil()
	assert.That(t, v).NotNil()
	assert.Number(t, v.id).Equal(1)

	// The second call skips the constructor and returns the cached value.
	v2, err := s.Init(func() (*resource, error) {
		calls++
		return &resource{id: 2}, nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, v2).Same(v)
	assert.Number(t, calls).Equal(1)
}

func TestInitCachesError(t *testing.T) {
	var s Singleton[int]
	boom := errors.New("boom")
	v, err := s.Init(func() (int, error) { return 5, boom })
	assert.Number(t, v).Equal(5)
	assert.Error(t, err).Is(boom)

	// The cached value comes back along with the cached error, even though
	// this constructor succeeds. The values differ (5 / 1 / zero) so each
	// return path is distinguishable.
	v2, err := s.Init(func() (int, error) { return 1, nil })
	assert.Number(t, v2).Equal(5)
	assert.Error(t, err).Is(boom)
}

func TestInitConcurrent(t *testing.T) {
	var s Singleton[int]
	var calls atomic.Int64
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range 100 {
		wg.Go(func() {
			<-start
			v, err := s.Init(func() (int, error) {
				calls.Add(1)
				return 42, nil
			})
			assert.Error(t, err).Nil()
			assert.Number(t, v).Equal(42)
		})
	}
	close(start)
	wg.Wait()
	assert.Number(t, calls.Load()).Equal(1)
}

func BenchmarkInit(b *testing.B) {
	var s Singleton[*resource]
	if _, err := s.Init(func() (*resource, error) { return &resource{id: 1}, nil }); err != nil {
		b.Fatal(err)
	}
	newFn := func() (*resource, error) { return nil, nil }
	b.ReportAllocs()
	for b.Loop() {
		if _, err := s.Init(newFn); err != nil {
			b.Fatal(err)
		}
	}
}
