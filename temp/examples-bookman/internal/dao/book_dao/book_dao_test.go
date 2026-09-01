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

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/testing/require"
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
	assert.Error(t, err).Nil()
	require.That(t, len(books)).Equal(1)
	assert.String(t, books[0].ISBN).Equal("978-0134190440")
	assert.String(t, books[0].Title).Equal("The Go Programming Language")

	// Test saving a new book
	err = dao.SaveBook(Book{
		Title:     "Clean Code",
		Author:    "Robert C. Martin",
		ISBN:      "978-0132350884",
		Publisher: "Prentice Hall",
	})
	assert.Error(t, err).Nil()

	// Verify book was added
	books, err = dao.ListBooks()
	assert.Error(t, err).Nil()
	assert.That(t, len(books)).Equal(2)

	// Test retrieving a book by ISBN
	book, err := dao.GetBook("978-0132350884")
	assert.Error(t, err).Nil()
	assert.String(t, book.Title).Equal("Clean Code")
	assert.String(t, book.Publisher).Equal("Prentice Hall")

	// Test retrieving a missing book
	_, err = dao.GetBook("missing")
	assert.Error(t, err).NotNil()

	// Test deleting a book
	err = dao.DeleteBook("978-0132350884")
	assert.Error(t, err).Nil()

	// Verify book was deleted
	books, err = dao.ListBooks()
	assert.Error(t, err).Nil()
	require.That(t, len(books)).Equal(1)
	assert.String(t, books[0].ISBN).Equal("978-0134190440")

	// Test saving a book without ISBN
	err = dao.SaveBook(Book{Title: "No ISBN"})
	assert.Error(t, err).NotNil()
}
