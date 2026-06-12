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

package iterutil

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func fibonacci(n int) int {
	if n <= 0 {
		return 0
	} else if n == 1 {
		return 1
	} else {
		return fibonacci(n-1) + fibonacci(n-2)
	}
}

func BenchmarkRanges(b *testing.B) {
	const N = 5

	b.Run("for", func(b *testing.B) {
		for b.Loop() {
			_ = fibonacci(N)
		}
	})

	b.Run("loop", func(b *testing.B) {
		for b.Loop() {
			Times(5, func(i int) {
				_ = fibonacci(N)
			})
		}
	})
}

func TestTimes(t *testing.T) {
	t.Run("positive count", func(t *testing.T) {
		var arr []int
		Times(5, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{0, 1, 2, 3, 4})
	})

	t.Run("zero count", func(t *testing.T) {
		var arr []int
		Times(0, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("negative count", func(t *testing.T) {
		var arr []int
		Times(-1, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})
}

func TestRanges(t *testing.T) {
	t.Run("forward range", func(t *testing.T) {
		var arr []int
		Ranges(1, 5, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{1, 2, 3, 4})
	})

	t.Run("backward range", func(t *testing.T) {
		var arr []int
		Ranges(5, 1, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{5, 4, 3, 2})
	})

	t.Run("equal start and end", func(t *testing.T) {
		var arr []int
		Ranges(3, 3, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("negative forward range", func(t *testing.T) {
		var arr []int
		Ranges(-3, 2, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{-3, -2, -1, 0, 1})
	})

	t.Run("negative backward range", func(t *testing.T) {
		var arr []int
		Ranges(2, -3, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{2, 1, 0, -1, -2})
	})
}

func TestStepRanges(t *testing.T) {
	t.Run("positive step", func(t *testing.T) {
		var arr []int
		StepRanges(1, 5, 2, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{1, 3})
	})

	t.Run("negative step", func(t *testing.T) {
		var arr []int
		StepRanges(5, 1, -2, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{5, 3})
	})

	t.Run("zero step", func(t *testing.T) {
		var arr []int
		StepRanges(1, 5, 0, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("step larger than range", func(t *testing.T) {
		var arr []int
		StepRanges(1, 5, 10, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{1})
	})

	t.Run("negative step with wrong direction", func(t *testing.T) {
		var arr []int
		StepRanges(1, 5, -1, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("positive step with wrong direction", func(t *testing.T) {
		var arr []int
		StepRanges(5, 1, 1, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("equal start and end with positive step", func(t *testing.T) {
		var arr []int
		StepRanges(3, 3, 1, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Nil()
	})

	t.Run("large negative step", func(t *testing.T) {
		var arr []int
		StepRanges(10, 0, -3, func(i int) {
			arr = append(arr, i)
		})
		assert.That(t, arr).Equal([]int{10, 7, 4, 1})
	})
}
