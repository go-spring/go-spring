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

package listutil

import (
	"container/list"
	"io"
)

// SliceOf returns a slice containing the provided items.
func SliceOf[T any](items ...T) []T {
	return items
}

// ListOf creates a new list.List and populates it with the provided items.
//
// All elements are inserted in the order they appear in the argument list.
func ListOf[T any](items ...T) *list.List {
	l := list.New()
	for _, item := range items {
		l.PushBack(item)
	}
	return l
}

// AllOfList returns all elements of the given list as a slice of type T.
//
// The caller must ensure that every element stored in the list is of type T.
// If an element has a different type, this function will panic at runtime.
//
// If the list is nil or empty, an empty slice is returned.
func AllOfList[T any](l *list.List) []T {
	if l == nil || l.Len() == 0 {
		return nil
	}
	ret := make([]T, 0, l.Len())
	for e := l.Front(); e != nil; e = e.Next() {
		ret = append(ret, e.Value.(T))
	}
	return ret
}

// WriteStrings writes the provided strings to w in order.
// It returns the first error encountered, if any.
func WriteStrings(w io.Writer, values ...string) error {
	for _, value := range values {
		if _, err := io.WriteString(w, value); err != nil {
			return err
		}
	}
	return nil
}
