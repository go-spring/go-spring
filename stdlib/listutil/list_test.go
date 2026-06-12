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
)

func TestList_BasicOperations(t *testing.T) {
	l := New[int]()

	// Test PushBack
	l.PushBack(1)
	e2 := l.PushBack(2)
	l.PushBack(3)

	if l.Len() != 3 {
		t.Errorf("expected length 3, got %d", l.Len())
	}

	// Test Front and Back
	if l.Front().Value() != 1 {
		t.Errorf("expected front value 1, got %d", l.Front().Value())
	}
	if l.Back().Value() != 3 {
		t.Errorf("expected back value 3, got %d", l.Back().Value())
	}

	// Test Remove
	val := l.Remove(e2)
	if val != 2 {
		t.Errorf("expected removed value 2, got %d", val)
	}
	if l.Len() != 2 {
		t.Errorf("expected length 2 after remove, got %d", l.Len())
	}
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
		if e.Value() != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], e.Value())
		}
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
	if l.Front().Value() != 3 {
		t.Errorf("expected front value 3 after MoveToFront, got %d", l.Front().Value())
	}

	// Test MoveToBack
	l.MoveToBack(e1)
	if l.Back().Value() != 1 {
		t.Errorf("expected back value 1 after MoveToBack, got %d", l.Back().Value())
	}

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
		if e.Value() != expected[i] {
			t.Errorf("expected %d, got %d", expected[i], e.Value())
		}
		i++
	}

	// Test PushFrontList
	l3 := New[int]()
	l3.PushBack(5)
	l3.PushBack(6)
	l3.PushFrontList(l1)

	if l3.Front().Value() != 1 {
		t.Errorf("expected front value 1 after PushFrontList, got %d", l3.Front().Value())
	}
}

func TestElement_Valid(t *testing.T) {
	l := New[int]()
	e := l.PushBack(1)

	if !e.Valid() {
		t.Error("expected element from list to be valid")
	}

	var empty Element[int]
	if empty.Valid() {
		t.Error("expected zero element to be invalid")
	}
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
