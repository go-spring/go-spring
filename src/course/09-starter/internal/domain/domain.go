package domain

import "context"

type Book struct {
	ISBN  string  `json:"isbn"`
	Title string  `json:"title"`
	Price float64 `json:"price"`
}

type BookRepository interface {
	List() []Book
	Find(isbn string) (Book, bool)
}

type PriceClient interface {
	GetPrice(ctx context.Context, isbn string) (float64, error)
}
