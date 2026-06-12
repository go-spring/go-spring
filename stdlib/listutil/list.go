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
)

// Element is an element of a linked list.
type Element[T any] struct {
	*list.Element
}

// Valid returns true if e is a valid element of list l.
func (e Element[T]) Valid() bool {
	return e.Element != nil
}

// Value returns the value of element e.
func (e Element[T]) Value() T {
	return e.Element.Value.(T)
}

// Next returns the next element of list l or nil if e is the last element.
func (e Element[T]) Next() Element[T] {
	return Element[T]{e.Element.Next()}
}

// Prev returns the previous element of list l or nil if e is the first element.
func (e Element[T]) Prev() Element[T] {
	return Element[T]{e.Element.Prev()}
}

// List is a doubly linked list.
type List[T any] struct {
	*list.List
}

// New returns an empty list.
func New[T any]() *List[T] {
	return &List[T]{List: list.New()}
}

// Len returns the number of elements of list l.
// The complexity is O(1).
func (l *List[T]) Len() int { return l.List.Len() }

// Front returns the first element of list l or nil if the list is empty.
func (l *List[T]) Front() Element[T] {
	return Element[T]{l.List.Front()}
}

// Back returns the last element of list l or nil if the list is empty.
func (l *List[T]) Back() Element[T] {
	return Element[T]{l.List.Back()}
}

// Remove removes e from l if e is an element of list l.
// It returns the element value e.Value.
// The element must not be nil.
func (l *List[T]) Remove(e Element[T]) T {
	return l.List.Remove(e.Element).(T)
}

// PushFront inserts a new element e with value v at the front of list l and returns e.
func (l *List[T]) PushFront(v T) Element[T] {
	return Element[T]{l.List.PushFront(v)}
}

// PushBack inserts a new element e with value v at the back of list l and returns e.
func (l *List[T]) PushBack(v T) Element[T] {
	return Element[T]{l.List.PushBack(v)}
}

// InsertBefore inserts a new element e with value v immediately before mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
func (l *List[T]) InsertBefore(v T, mark Element[T]) Element[T] {
	return Element[T]{l.List.InsertBefore(v, mark.Element)}
}

// InsertAfter inserts a new element e with value v immediately after mark and returns e.
// If mark is not an element of l, the list is not modified.
// The mark must not be nil.
func (l *List[T]) InsertAfter(v T, mark Element[T]) Element[T] {
	return Element[T]{l.List.InsertAfter(v, mark.Element)}
}

// MoveToFront moves element e to the front of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *List[T]) MoveToFront(e Element[T]) {
	l.List.MoveToFront(e.Element)
}

// MoveToBack moves element e to the back of list l.
// If e is not an element of l, the list is not modified.
// The element must not be nil.
func (l *List[T]) MoveToBack(e Element[T]) {
	l.List.MoveToBack(e.Element)
}

// MoveBefore moves element e to its new position before mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List[T]) MoveBefore(e, mark Element[T]) {
	l.List.MoveBefore(e.Element, mark.Element)
}

// MoveAfter moves element e to its new position after mark.
// If e or mark is not an element of l, or e == mark, the list is not modified.
// The element and mark must not be nil.
func (l *List[T]) MoveAfter(e, mark Element[T]) {
	l.List.MoveAfter(e.Element, mark.Element)
}

// PushBackList inserts a copy of another list at the back of list l.
// The lists l and other may be the same. They must not be nil.
func (l *List[T]) PushBackList(other *List[T]) {
	l.List.PushBackList(other.List)
}

// PushFrontList inserts a copy of another list at the front of list l.
// The lists l and other may be the same. They must not be nil.
func (l *List[T]) PushFrontList(other *List[T]) {
	l.List.PushFrontList(other.List)
}
