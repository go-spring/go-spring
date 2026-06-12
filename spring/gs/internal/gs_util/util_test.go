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

package gs_util

import (
	"container/list"
	"testing"

	"github.com/go-spring/stdlib/listutil"
	"github.com/go-spring/stdlib/testing/assert"
)

func TestTopologicalSort(t *testing.T) {

	t.Run("empty list", func(t *testing.T) {
		sorting := list.New()
		sorted, err := TopologicalSort(sorting, nil)
		assert.That(t, err).Nil()
		assert.That(t, sorted.Len()).Equal(0)
	})

	t.Run("single element", func(t *testing.T) {
		getBefore := func(_ *list.List, _ any) *list.List {
			return list.New()
		}
		sorting := listutil.ListOf("A")
		sorted, err := TopologicalSort(sorting, getBefore)
		assert.That(t, err).Nil()
		assert.That(t, sorted.Len()).Equal(1)
		assert.That(t, sorted.Front().Value).Equal("A")
	})

	t.Run("independent elements", func(t *testing.T) {
		// A、B、C
		getBefore := func(_ *list.List, _ any) *list.List {
			return list.New()
		}
		sorting := listutil.ListOf("A", "B", "C")
		sorted, err := TopologicalSort(sorting, getBefore)
		assert.That(t, err).Nil()
		assert.That(t, listutil.AllOfList[string](sorted)).Equal([]string{"A", "B", "C"})
	})

	t.Run("linear dependency", func(t *testing.T) {
		// A -> B -> C
		getBefore := func(_ *list.List, current any) *list.List {
			l := list.New()
			switch current {
			case "A":
				l.PushBack("B")
			case "B":
				l.PushBack("C")
			}
			return l
		}
		sorting := listutil.ListOf("A", "B", "C")
		sorted, err := TopologicalSort(sorting, getBefore)
		assert.That(t, err).Nil()
		assert.That(t, listutil.AllOfList[string](sorted)).Equal([]string{"C", "B", "A"})
	})

	t.Run("multiple dependencies", func(t *testing.T) {
		// A -> B&C, B -> C
		getBefore := func(_ *list.List, current any) *list.List {
			l := list.New()
			switch current {
			case "A":
				l.PushBack("B")
				l.PushBack("C")
			case "B":
				l.PushBack("C")
			}
			return l
		}
		sorting := listutil.ListOf("A", "B", "C")
		sorted, err := TopologicalSort(sorting, getBefore)
		assert.That(t, err).Nil()
		assert.That(t, listutil.AllOfList[string](sorted)).Equal([]string{"C", "B", "A"})
	})

	t.Run("cycle", func(t *testing.T) {
		// A -> B -> C -> A
		getBefore := func(_ *list.List, current any) *list.List {
			l := list.New()
			switch current {
			case "A":
				l.PushBack("B")
			case "B":
				l.PushBack("C")
			case "C":
				l.PushBack("A") // cycle
			}
			return l
		}
		sorting := listutil.ListOf("A", "B", "C")
		_, err := TopologicalSort(sorting, getBefore)
		assert.Error(t, err).Matches("dependency cycle detected")
	})
}
