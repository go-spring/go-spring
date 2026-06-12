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

package gs_util

import (
	"container/list"

	"github.com/go-spring/stdlib/errutil"
)

// GetBeforeItems is a function type that returns a list of items
// that the given current item depends on (must be processed before current item).
type GetBeforeItems func(items *list.List, current any) *list.List

// TopologicalSort performs topological sorting using three lists (processing, toSort, sorted)
// to resolve dependencies and return a sorted list.
// The input `items` is a list of all items to be sorted, and `fn` determines dependencies.
func TopologicalSort(items *list.List, fn GetBeforeItems) (*list.List, error) {
	toSort := list.New()     // List of items that still need to be sorted.
	sorted := list.New()     // List of items that have been fully sorted.
	processing := list.New() // List of items currently being processed.

	// Initialize the toSort list with all elements.
	toSort.PushBackList(items)

	// Process items in the toSort list until all items are sorted.
	for toSort.Len() > 0 {
		// Recursively sort the dependency chain starting with the next item in `toSort`.
		err := dfsTopoVisit(items, toSort, sorted, processing, nil, fn)
		if err != nil {
			return nil, err
		}
	}
	return sorted, nil
}

// searchInList searches for an element `v` in the list `l`.
// If the element exists, it returns a pointer to the list element. Otherwise, it returns nil.
func searchInList(l *list.List, v any) *list.Element {
	for e := l.Front(); e != nil; e = e.Next() {
		if e.Value == v {
			return e
		}
	}
	return nil
}

// dfsTopoVisit recursively processes the current item and its dependencies using DFS.
// Parameters:
// - items: The original list of items.
// - toSort: The list of items to be sorted.
// - sorted: The list of items that have been sorted.
// - processing: The list of items currently being processed (to detect cycles).
// - current: The current item being processed (nil for the first item).
// - fn: A function that retrieves the list of items that must appear before the current item.
func dfsTopoVisit(items *list.List, toSort *list.List, sorted *list.List,
	processing *list.List, current any, fn GetBeforeItems) error {

	// If no current item is specified, take the first item from the `toSort` list for processing.
	if current == nil {
		current = toSort.Remove(toSort.Front())
	}

	// Retrieve dependencies for the current item.
	l := fn(items, current)

	// Add the current item to the processing list to mark it as being processed.
	processing.PushBack(current)

	// Process dependencies for the current item.
	for e := l.Front(); e != nil; e = e.Next() {
		c := e.Value

		// Detect circular dependencies by checking if `c` is already being processed.
		if searchInList(processing, c) != nil {
			return errutil.Explain(nil, "dependency cycle detected")
		}

		// Check if the dependency `c` is already sorted or still in the toSort list.
		inSorted := searchInList(sorted, c) != nil
		inToSort := searchInList(toSort, c) != nil

		// If the dependency is not sorted but still needs sorting, process it recursively.
		if !inSorted && inToSort {
			err := dfsTopoVisit(items, toSort, sorted, processing, c, fn)
			if err != nil {
				return err
			}
		}
	}

	// Remove the current item from the processing list.
	if e := searchInList(processing, current); e != nil {
		processing.Remove(e)
	}

	// Remove the current item from the toSort list (if it is still there).
	if e := searchInList(toSort, current); e != nil {
		toSort.Remove(e)
	}

	// Add the current item to the sorted list to mark it as fully processed.
	sorted.PushBack(current)
	return nil
}
