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

package book_dao

import (
	"testing"
)

func TestBookDao(t *testing.T) {
	dao := &BookDao{Store: map[string]Book{
		"978-0134190440": {
			Title:     "The Go Programming Language",
			Author:    "Alan A. A. Donovan, Brian W. Kernighan",
			ISBN:      "978-0134190440",
			Publisher: "Addison-Wesley",
		},
	}}

	// Test listing books
	books, err := dao.ListBooks()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].ISBN != "978-0134190440" {
		t.Fatalf("expected ISBN 978-0134190440, got %s", books[0].ISBN)
	}
	if books[0].Title != "The Go Programming Language" {
		t.Fatalf("expected title 'The Go Programming Language', got %s", books[0].Title)
	}

	// Test saving a new book
	err = dao.SaveBook(Book{
		Title:     "Clean Code",
		Author:    "Robert C. Martin",
		ISBN:      "978-0132350884",
		Publisher: "Prentice Hall",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify book was added
	books, err = dao.ListBooks()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books, got %d", len(books))
	}

	// Test retrieving a book by ISBN
	book, err := dao.GetBook("978-0132350884")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if book.Title != "Clean Code" {
		t.Fatalf("expected title 'Clean Code', got %s", book.Title)
	}
	if book.Publisher != "Prentice Hall" {
		t.Fatalf("expected publisher 'Prentice Hall', got %s", book.Publisher)
	}

	// Test retrieving a missing book
	_, err = dao.GetBook("missing")
	if err == nil {
		t.Fatal("expected missing book error")
	}

	// Test deleting a book
	err = dao.DeleteBook("978-0132350884")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify book was deleted
	books, err = dao.ListBooks()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book after deletion, got %d", len(books))
	}
	if books[0].ISBN != "978-0134190440" {
		t.Fatalf("expected ISBN 978-0134190440 after deletion, got %s", books[0].ISBN)
	}

	// Test saving a book without ISBN
	err = dao.SaveBook(Book{Title: "No ISBN"})
	if err == nil {
		t.Fatal("expected missing isbn error")
	}
}
