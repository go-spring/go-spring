package main

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/go-spring/spring-core/gs"
)

type Book struct {
	ISBN  string `json:"isbn"`
	Title string `json:"title"`
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
		"978-0134190440": {ISBN: "978-0134190440", Title: "The Go Programming Language"},
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

type FakeMysqlBookDao struct {
	*MemoryBookDao
}

func NewFakeMysqlBookDao() *FakeMysqlBookDao {
	return &FakeMysqlBookDao{MemoryBookDao: NewMemoryBookDao()}
}

type BookCache struct {
	Enabled bool          `value:"${bookman.cache.enabled:=false}"`
	TTL     time.Duration `value:"${bookman.cache.ttl:=30s}"`
	mu      sync.Mutex
	items   map[string]cacheItem
}

type cacheItem struct {
	book      Book
	expiresAt time.Time
}

func NewBookCache() *BookCache {
	return &BookCache{items: make(map[string]cacheItem)}
}

func (c *BookCache) Get(isbn string) (Book, bool) {
	if !c.Enabled {
		return Book{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[isbn]
	if !ok || time.Now().After(item.expiresAt) {
		delete(c.items, isbn)
		return Book{}, false
	}
	return item.book, true
}

func (c *BookCache) Set(book Book) {
	if !c.Enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[book.ISBN] = cacheItem{book: book, expiresAt: time.Now().Add(c.TTL)}
}

type BookService struct {
	repo  BookRepository
	cache *BookCache
}

func NewBookService(repo BookRepository, cache *BookCache) *BookService {
	return &BookService{repo: repo, cache: cache}
}

func (s *BookService) List() []Book {
	return s.repo.List()
}

func (s *BookService) Find(isbn string) (Book, bool) {
	if book, ok := s.cache.Get(isbn); ok {
		return book, true
	}
	book, ok := s.repo.Find(isbn)
	if ok {
		s.cache.Set(book)
	}
	return book, ok
}

type BookController struct {
	Service *BookService `autowire:""`
	PProf   bool         `value:"${pprof.enable:=false}"`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.List())
}

func (c *BookController) Get(w http.ResponseWriter, r *http.Request) {
	book, ok := c.Service.Find(r.PathValue("isbn"))
	if !ok {
		http.Error(w, "book not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(book)
}

func (c *BookController) PProfIndex(w http.ResponseWriter, r *http.Request) {
	if !c.PProf {
		http.NotFound(w, r)
		return
	}
	_, _ = w.Write([]byte("pprof is enabled in this course example"))
}

func init() {
	gs.Provide(NewMemoryBookDao).
		Condition(gs.OnProperty("bookman.dao.type").HavingValue("memory").MatchIfMissing()).
		Export(gs.As[BookRepository]())
	gs.Provide(NewFakeMysqlBookDao).
		Condition(gs.OnProperty("bookman.dao.type").HavingValue("mysql")).
		Export(gs.As[BookRepository]())
	gs.Provide(NewBookCache)
	gs.Provide(NewBookService)
	gs.Provide(&BookController{})
	gs.Provide(func(c *BookController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /books", c.List)
		mux.HandleFunc("GET /books/{isbn}", c.Get)
		mux.HandleFunc("GET /debug/pprof/", c.PProfIndex)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
