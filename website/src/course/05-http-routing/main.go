package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"

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
	Save(book Book)
	Delete(isbn string) bool
}

type MemoryBookDao struct {
	mu    sync.RWMutex
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
	d.mu.RLock()
	defer d.mu.RUnlock()
	books := make([]Book, 0, len(d.books))
	for _, book := range d.books {
		books = append(books, book)
	}
	return books
}

func (d *MemoryBookDao) Find(isbn string) (Book, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	book, ok := d.books[isbn]
	return book, ok
}

func (d *MemoryBookDao) Save(book Book) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.books[book.ISBN] = book
}

func (d *MemoryBookDao) Delete(isbn string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.books[isbn]; !ok {
		return false
	}
	delete(d.books, isbn)
	return true
}

type BookService struct {
	repo      BookRepository
	Overwrite bool `value:"${bookman.book.overwrite:=true}"`
}

func NewBookService(repo BookRepository) *BookService {
	return &BookService{repo: repo}
}

func (s *BookService) ListBooks() []Book {
	return s.repo.List()
}

func (s *BookService) GetBook(isbn string) (Book, bool) {
	return s.repo.Find(isbn)
}

func (s *BookService) SaveBook(book Book) error {
	if book.ISBN == "" {
		return errors.New("isbn is required")
	}
	if book.Title == "" {
		return errors.New("title is required")
	}
	if !s.Overwrite {
		if _, ok := s.repo.Find(book.ISBN); ok {
			return errors.New("book already exists")
		}
	}
	s.repo.Save(book)
	return nil
}

func (s *BookService) DeleteBook(isbn string) bool {
	return s.repo.Delete(isbn)
}

type BookController struct {
	Service *BookService `autowire:""`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.ListBooks())
}

func (c *BookController) Get(w http.ResponseWriter, r *http.Request) {
	book, ok := c.Service.GetBook(r.PathValue("isbn"))
	if !ok {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(book)
}

func (c *BookController) Save(w http.ResponseWriter, r *http.Request) {
	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := c.Service.SaveBook(book); err != nil {
		if err.Error() == "book already exists" {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *BookController) Delete(w http.ResponseWriter, r *http.Request) {
	if !c.Service.DeleteBook(r.PathValue("isbn")) {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func accessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}

func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[BookRepository]())
	gs.Provide(NewBookService)
	gs.Provide(&BookController{})
	gs.Provide(func(c *BookController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /books", c.List)
		mux.HandleFunc("GET /books/{isbn}", c.Get)
		mux.HandleFunc("POST /books", c.Save)
		mux.HandleFunc("DELETE /books/{isbn}", c.Delete)
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("BookMan Pro"))
		})
		return &gs.HttpServeMux{Handler: accessLog(mux)}
	})
}

func main() {
	gs.Run()
}
