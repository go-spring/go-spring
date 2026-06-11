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

package book_service

import (
	"testing"

	"bookman/internal/dao/book_dao"

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/stdlib/testing/assert"
	"github.com/go-spring/stdlib/testing/require"
)

func TestBookService(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Service *BookService `autowire:""`
	}) {
		// BookDao 不应包含状态，因为它是全局共享对象
		s.Service.BookDao = &book_dao.BookDao{Store: map[string]book_dao.Book{
			"978-0134190440": {
				Title:     "The Go Programming Language",
				Author:    "Alan A. A. Donovan, Brian W. Kernighan",
				ISBN:      "978-0134190440",
				Publisher: "Addison-Wesley",
			},
		}}

		ctx := t.Context()

		// Test listing books
		books, err := s.Service.ListBooks(ctx)
		require.That(t, err).Nil()
		assert.Slice(t, books).Length(1)
		assert.That(t, books[0].ISBN).Equal("978-0134190440")

		// Test saving a new book
		err = s.Service.SaveBook(ctx, book_dao.Book{
			Title:     "Introduction to Algorithms",
			Author:    "Thomas H. Cormen, Charles E. Leiserson, ...",
			ISBN:      "978-0262033848",
			Publisher: "MIT Press",
		})
		require.That(t, err).Nil()

		// Verify book was added successfully
		books, err = s.Service.ListBooks(ctx)
		require.That(t, err).Nil()
		assert.Slice(t, books).Length(2)

		// Test retrieving a book by ISBN
		book, err := s.Service.GetBook(ctx, "978-0134190440")
		require.That(t, err).Nil()
		assert.That(t, book.ISBN).Equal("978-0134190440")
		assert.That(t, book.Title).Equal("The Go Programming Language")

		// Test deleting a book
		err = s.Service.DeleteBook(ctx, "978-0134190440")
		require.That(t, err).Nil()

		// Verify book deletion
		books, err = s.Service.ListBooks(ctx)
		require.That(t, err).Nil()
		assert.Slice(t, books).Length(1)
	})
}
