package main

import (
	"context"
	"encoding/json"
	"net/http"

	"bookman-pro-09/internal/domain"

	"github.com/go-spring/spring-core/gs"

	_ "bookman-pro-09/starter/bookprice"
)

type MemoryBookDao struct {
	books map[string]domain.Book
}

func NewMemoryBookDao() *MemoryBookDao {
	return &MemoryBookDao{books: map[string]domain.Book{
		"978-0134190440": {ISBN: "978-0134190440", Title: "The Go Programming Language"},
	}}
}

func (d *MemoryBookDao) List() []domain.Book {
	books := make([]domain.Book, 0, len(d.books))
	for _, book := range d.books {
		books = append(books, book)
	}
	return books
}

func (d *MemoryBookDao) Find(isbn string) (domain.Book, bool) {
	book, ok := d.books[isbn]
	return book, ok
}

type FixedPriceClient struct{}

func (c *FixedPriceClient) GetPrice(ctx context.Context, isbn string) (float64, error) {
	return 9.9, nil
}

type BookService struct {
	repo  domain.BookRepository
	price domain.PriceClient
}

func NewBookService(repo domain.BookRepository, price domain.PriceClient) *BookService {
	return &BookService{repo: repo, price: price}
}

func (s *BookService) List(ctx context.Context) []domain.Book {
	books := s.repo.List()
	for i := range books {
		books[i].Price, _ = s.price.GetPrice(ctx, books[i].ISBN)
	}
	return books
}

type BookController struct {
	Service *BookService `autowire:""`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.List(r.Context()))
}

func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[domain.BookRepository]())
	gs.Provide(&FixedPriceClient{}).
		Condition(gs.OnMissingBean[domain.PriceClient]()).
		Export(gs.As[domain.PriceClient]())
	gs.Provide(NewBookService)
	gs.Provide(&BookController{})
	gs.Provide(func(c *BookController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/books", c.List)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
