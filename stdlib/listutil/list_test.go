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
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestList_BasicOperations(t *testing.T) {
	l := New[int]()

	// Test PushBack
	l.PushBack(1)
	e2 := l.PushBack(2)
	l.PushBack(3)

	assert.That(t, l.Len()).Equal(3)

	// Test Front and Back
	assert.That(t, l.Front().Value()).Equal(1)
	assert.That(t, l.Back().Value()).Equal(3)

	// Test Remove
	val := l.Remove(e2)
	assert.That(t, val).Equal(2)
	assert.That(t, l.Len()).Equal(2)
}

func TestList_InsertOperations(t *testing.T) {
	l := New[string]()

	e1 := l.PushBack("first")
	e3 := l.PushBack("third")

	// Test InsertBefore
	l.InsertBefore("second", e3)

	// Test InsertAfter
	l.InsertAfter("fourth", e3)

	expected := []string{"first", "second", "third", "fourth"}
	i := 0
	for e := l.Front(); e.Valid(); e = e.Next() {
		assert.That(t, e.Value()).Equal(expected[i])
		i++
	}
	_ = e1
}

func TestList_MoveOperations(t *testing.T) {
	l := New[int]()

	e1 := l.PushBack(1)
	e2 := l.PushBack(2)
	e3 := l.PushBack(3)

	// Test MoveToFront
	l.MoveToFront(e3)
	assert.That(t, l.Front().Value()).Equal(3)

	// Test MoveToBack
	l.MoveToBack(e1)
	assert.That(t, l.Back().Value()).Equal(1)

	// Test MoveBefore
	l.MoveBefore(e2, e1)

	// Test MoveAfter
	l.MoveAfter(e3, e1)
}

func TestList_ListOperations(t *testing.T) {
	l1 := New[int]()
	l1.PushBack(1)
	l1.PushBack(2)

	l2 := New[int]()
	l2.PushBack(3)
	l2.PushBack(4)

	// Test PushBackList
	l1.PushBackList(l2)

	expected := []int{1, 2, 3, 4}
	i := 0
	for e := l1.Front(); e.Valid(); e = e.Next() {
		assert.That(t, e.Value()).Equal(expected[i])
		i++
	}

	// Test PushFrontList
	l3 := New[int]()
	l3.PushBack(5)
	l3.PushBack(6)
	l3.PushFrontList(l1)

	assert.That(t, l3.Front().Value()).Equal(1)
}

func TestElement_Valid(t *testing.T) {
	l := New[int]()
	e := l.PushBack(1)

	assert.That(t, e.Valid()).True()

	var empty Element[int]
	assert.That(t, empty.Valid()).False()
}

func ExampleNew() {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	l.PushBack(3)

	for e := l.Front(); e.Valid(); e = e.Next() {
		_ = e.Value()
	}
	// Output:
}

func ExampleList_PushBack() {
	l := New[string]()
	l.PushBack("apple")
	l.PushBack("banana")
	l.PushBack("cherry")

	front := l.Front().Value()
	back := l.Back().Value()
	_ = front
	_ = back
	// Output:
}

func ExampleList_Remove() {
	l := New[int]()
	e1 := l.PushBack(10)
	e2 := l.PushBack(20)
	e3 := l.PushBack(30)

	l.Remove(e2)

	for e := l.Front(); e.Valid(); e = e.Next() {
		_ = e.Value()
	}
	_ = e1
	_ = e3
	// Output:
}

func ExampleList_InsertBefore() {
	l := New[int]()
	e1 := l.PushBack(1)
	e3 := l.PushBack(3)

	l.InsertBefore(2, e3)

	for e := l.Front(); e.Valid(); e = e.Next() {
		_ = e.Value()
	}
	_ = e1
	// Output:
}

func ExampleList_MoveToFront() {
	l := New[int]()
	l.PushBack(1)
	l.PushBack(2)
	l.PushBack(3)

	e := l.Back()
	l.MoveToFront(e)

	_ = l.Front().Value()
	// Output:
}

func ExampleList_PushBackList() {
	l1 := New[int]()
	l1.PushBack(1)
	l1.PushBack(2)

	l2 := New[int]()
	l2.PushBack(3)
	l2.PushBack(4)

	l1.PushBackList(l2)

	for e := l1.Front(); e.Valid(); e = e.Next() {
		_ = e.Value()
	}
	// Output:
}
