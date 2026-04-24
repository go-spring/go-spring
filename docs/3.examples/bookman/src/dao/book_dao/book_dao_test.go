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

//func init() {
//	_ = os.Setenv("GS_SPRING_APP_CONFIG-LOCAL_DIR", "../../../conf")
//}
//
//func TestMain(m *testing.M) {
//	gstest.TestMain(m)
//}
//
//func TestBookDao(t *testing.T) {
//
//	// Wire dependencies and retrieve the server address
//	x := gstest.Wire(t, &struct {
//		SvrAddr string `value:"${server.addr}"`
//	}{})
//	assert.That(t, x.SvrAddr).Equal("0.0.0.0:9090")
//
//	// Retrieve BookDao instance
//	o := gstest.Get[*BookDao](t)
//	assert.NotNil(t, o)
//
//	// Test listing books
//	books, err := o.ListBooks()
//	assert.Nil(t, err)
//	assert.That(t, len(books)).Equal(1)
//	assert.That(t, books[0].ISBN).Equal("978-0134190440")
//	assert.That(t, books[0].Title).Equal("The Go Programming Language")
//
//	// Test saving a new book
//	err = o.SaveBook(Book{
//		Title:     "Clean Code",
//		Author:    "Robert C. Martin",
//		ISBN:      "978-0132350884",
//		Publisher: "Prentice Hall",
//	})
//	assert.That(t, err).Equal(nil)
//
//	// Verify book was added
//	books, err = o.ListBooks()
//	assert.Nil(t, err)
//	assert.That(t, len(books)).Equal(2)
//	assert.That(t, books[0].ISBN).Equal("978-0132350884")
//	assert.That(t, books[0].Title).Equal("Clean Code")
//
//	// Test retrieving a book by ISBN
//	book, err := o.GetBook("978-0132350884")
//	assert.Nil(t, err)
//	assert.That(t, book.Title).Equal("Clean Code")
//	assert.That(t, book.Publisher).Equal("Prentice Hall")
//
//	// Test deleting a book
//	err = o.DeleteBook("978-0132350884")
//	assert.Nil(t, err)
//
//	// Verify book was deleted
//	books, err = o.ListBooks()
//	assert.Nil(t, err)
//	assert.That(t, len(books)).Equal(1)
//	assert.That(t, books[0].ISBN).Equal("978-0134190440")
//}
