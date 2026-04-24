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
	"fmt"
	"log/slog"
	"strconv"

	"bookman/src/dao/book_dao"
	"bookman/src/idl/http/proto"
	"bookman/src/sdk/book_sdk"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(&BookService{})
}

type BookService struct {
	BookDao     *book_dao.BookDao `autowire:""`
	BookSDK     *book_sdk.BookSDK `autowire:""`
	Logger      *slog.Logger      `autowire:"biz"`
	RefreshTime gs.Dync[int64]    `value:"${refresh_time:=0}"`
}

// ListBooks retrieves all books from the database and enriches them with
// pricing and refresh time.
func (s *BookService) ListBooks() ([]proto.Book, error) {
	books, err := s.BookDao.ListBooks()
	if err != nil {
		s.Logger.Error(fmt.Sprintf("ListBooks return err: %s", err.Error()))
		return nil, err
	}
	ret := make([]proto.Book, 0, len(books))
	for _, book := range books {
		ret = append(ret, proto.Book{
			ISBN:        book.ISBN,
			Title:       book.Title,
			Author:      book.Author,
			Publisher:   book.Publisher,
			Price:       s.BookSDK.GetPrice(book.ISBN),
			RefreshTime: strconv.FormatInt(s.RefreshTime.Value(), 10),
		})
	}
	return ret, nil
}

// GetBook retrieves a single book by its ISBN and enriches it with
// pricing and refresh time.
func (s *BookService) GetBook(isbn string) (proto.Book, error) {
	book, err := s.BookDao.GetBook(isbn)
	if err != nil {
		s.Logger.Error(fmt.Sprintf("GetBook return err: %s", err.Error()))
		return proto.Book{}, err
	}
	return proto.Book{
		ISBN:        book.ISBN,
		Title:       book.Title,
		Author:      book.Author,
		Publisher:   book.Publisher,
		Price:       s.BookSDK.GetPrice(book.ISBN),
		RefreshTime: strconv.FormatInt(s.RefreshTime.Value(), 10),
	}, nil
}

// SaveBook stores a new book in the database.
func (s *BookService) SaveBook(book book_dao.Book) error {
	return s.BookDao.SaveBook(book)
}

// DeleteBook removes a book from the database by its ISBN.
func (s *BookService) DeleteBook(isbn string) error {
	return s.BookDao.DeleteBook(isbn)
}
