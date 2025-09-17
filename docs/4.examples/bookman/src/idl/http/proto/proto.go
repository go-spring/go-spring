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

// Package proto defines the interfaces and route registrations generated from IDL files.
package proto

import (
	"net/http"
)

// Book represents the structure of a book entity.
type Book struct {
	Title       string `json:"title"`
	Author      string `json:"author"`
	ISBN        string `json:"isbn"`
	Publisher   string `json:"publisher"`
	Price       string `json:"price"`
	RefreshTime string `json:"refreshTime"`
}

// Controller defines the service interface for book-related operations.
type Controller interface {
	ListBooks(w http.ResponseWriter, r *http.Request)
	GetBook(w http.ResponseWriter, r *http.Request)
	SaveBook(w http.ResponseWriter, r *http.Request)
	DeleteBook(w http.ResponseWriter, r *http.Request)
}

// RegisterRouter registers the HTTP routes for the Controller interface.
// It maps each method to its corresponding HTTP endpoint,
// and applies the given middleware (wrap) to each handler.
func RegisterRouter(mux *http.ServeMux, c Controller, wrap func(next http.Handler) http.Handler) {
	mux.Handle("GET /books", wrap(http.HandlerFunc(c.ListBooks)))
	mux.Handle("GET /books/{isbn}", wrap(http.HandlerFunc(c.GetBook)))
	mux.Handle("POST /books", wrap(http.HandlerFunc(c.SaveBook)))
	mux.Handle("DELETE /books/{isbn}", wrap(http.HandlerFunc(c.DeleteBook)))
}
