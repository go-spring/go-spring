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

// Times executes the function 'fn' exactly 'count' times.
// Used to eliminate deferred execution under standard for loops.
func Times(count int, fn func(i int)) {
	for i := range count {
		fn(i)
	}
}

// Ranges iterates from 'start' to 'end' (exclusive) and applies 'fn' to each index.
// Used to eliminate deferred execution under standard for loops.
func Ranges(start, end int, fn func(i int)) {
	if start < end {
		stepRangesForward(start, end, 1, fn)
	} else {
		stepRangesBackward(start, end, -1, fn)
	}
}

// StepRanges iterates from 'start' to 'end' using a step size and applies 'fn' to each index.
// Used to eliminate deferred execution under standard for loops.
func StepRanges(start, end, step int, fn func(i int)) {
	if step > 0 && start < end {
		stepRangesForward(start, end, step, fn)
	} else if step < 0 && start > end {
		stepRangesBackward(start, end, step, fn)
	}
}

// stepRangesForward helper function for forward step iteration.
func stepRangesForward(start, end, step int, fn func(i int)) {
	for i := start; i < end; i += step {
		fn(i)
	}
}

// stepRangesBackward helper function for backward step iteration.
func stepRangesBackward(start, end, step int, fn func(i int)) {
	for i := start; i > end; i += step {
		fn(i)
	}
}
