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

package gs_app

import (
	"sync"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
)

func TestReadySignal(t *testing.T) {

	t.Run("zero workers", func(t *testing.T) {
		signal := NewReadySignal()
		signal.Wait()
		assert.That(t, signal.Intercepted()).False()
	})

	t.Run("intercept", func(t *testing.T) {
		const workers = 3

		signal := NewReadySignal()
		for i := range workers {
			num := i
			workerSignal := signal.Add()
			go func() {
				if num == 0 {
					workerSignal.Intercept()
				} else {
					<-workerSignal.TriggerAndWait()
				}
			}()
		}

		signal.Wait()
		assert.That(t, signal.Intercepted()).True()
	})

	t.Run("multiple intercept", func(t *testing.T) {
		const workers = 3

		signal := NewReadySignal()
		for i := range workers {
			workerSignal := signal.Add()
			go func(num int) {
				if num < 2 {
					workerSignal.Intercept()
				} else {
					<-workerSignal.TriggerAndWait()
				}
			}(i)
		}

		signal.Wait()
		assert.That(t, signal.Intercepted()).True()
	})

	t.Run("intercept after ready", func(t *testing.T) {
		signal := NewReadySignal()
		workerSignal := signal.Add()
		_ = workerSignal.TriggerAndWait()
		workerSignal.Intercept()
		signal.Wait()
		assert.That(t, signal.Intercepted()).True()
	})

	t.Run("duplicate trigger only counts once", func(t *testing.T) {
		signal := NewReadySignal()
		workerSignal := signal.Add()
		otherSignal := signal.Add()
		_ = workerSignal.TriggerAndWait()
		_ = workerSignal.TriggerAndWait()

		waitDone := make(chan struct{})
		go func() {
			signal.Wait()
			close(waitDone)
		}()

		select {
		case <-waitDone:
			t.Fatal("duplicate trigger released another signal")
		case <-time.After(10 * time.Millisecond):
		}
		assert.That(t, signal.Intercepted()).False()
		otherSignal.Intercept()
		<-waitDone
	})

	t.Run("success", func(t *testing.T) {
		const workers = 3

		var wg sync.WaitGroup
		wg.Add(workers)
		defer wg.Wait()

		signal := NewReadySignal()
		for range workers {
			workerSignal := signal.Add()
			go func() {
				defer wg.Done()
				<-workerSignal.TriggerAndWait()
			}()
		}

		signal.Wait()
		assert.That(t, signal.Intercepted()).False()

		signal.Close()
	})
}
