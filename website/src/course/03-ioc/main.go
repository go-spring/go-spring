package main

import (
	"encoding/json"
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

type Book struct {
	ISBN      string `json:"isbn"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Publisher string `json:"publisher"`
}

type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}

type MemoryBookDao struct {
	books map[string]Book
}

func NewMemoryBookDao() *MemoryBookDao {
	return &MemoryBookDao{books: map[string]Book{
		"978-0134190440": {
			ISBN: "978-0134190440", Title: "The Go Programming Language",
			Author: "Alan A. A. Donovan", Publisher: "Addison-Wesley",
		},
	}}
}

func (d *MemoryBookDao) List() []Book {
	books := make([]Book, 0, len(d.books))
	for _, book := range d.books {
		books = append(books, book)
	}
	return books
}

func (d *MemoryBookDao) Find(isbn string) (Book, bool) {
	book, ok := d.books[isbn]
	return book, ok
}

type BookService struct {
	repo BookRepository
}

func NewBookService(repo BookRepository) *BookService {
	return &BookService{repo: repo}
}

func (s *BookService) ListBooks() []Book {
	return s.repo.List()
}

func (s *BookService) CountBooks() int {
	return len(s.repo.List())
}

type BookController struct {
	Service *BookService `autowire:""`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.ListBooks())
}

func (c *BookController) Count(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]int{"count": c.Service.CountBooks()})
}

func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[BookRepository]())
	gs.Provide(NewBookService)
	gs.Provide(&BookController{})
	gs.Provide(func(c *BookController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/books", c.List)
		mux.HandleFunc("/books/count", c.Count)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
