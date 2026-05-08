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
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Provide(&BookDao{Store: map[string]Book{
		"978-0134190440": {
			Title:     "The Go Programming Language",
			Author:    "Alan A. A. Donovan, Brian W. Kernighan",
			ISBN:      "978-0134190440",
			Publisher: "Addison-Wesley",
		},
	}})
}

type Book struct {
	Title     string `json:"title"`
	Author    string `json:"author"`
	ISBN      string `json:"isbn"`
	Publisher string `json:"publisher"`
}

type BookDao struct {
	Store map[string]Book
}

// ListBooks returns a sorted list of all books in the store.
func (dao *BookDao) ListBooks() ([]Book, error) {
	r := slices.Collect(maps.Values(dao.Store))
	sort.Slice(r, func(i, j int) bool {
		return r[i].ISBN < r[j].ISBN
	})
	return r, nil
}

// GetBook retrieves a book by its ISBN.
func (dao *BookDao) GetBook(isbn string) (Book, error) {
	r, ok := dao.Store[isbn]
	if !ok {
		return Book{}, fmt.Errorf("book %q not found", isbn)
	}
	return r, nil
}

// SaveBook adds or updates a book in the store.
func (dao *BookDao) SaveBook(book Book) error {
	if book.ISBN == "" {
		return errors.New("book isbn is required")
	}
	dao.Store[book.ISBN] = book
	return nil
}

// DeleteBook removes a book from the store by its ISBN.
func (dao *BookDao) DeleteBook(isbn string) error {
	if _, ok := dao.Store[isbn]; !ok {
		return fmt.Errorf("book %q not found", isbn)
	}
	delete(dao.Store, isbn)
	return nil
}
