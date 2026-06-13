package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"go-spring.org/spring/gs"
)

type Book struct {
	ISBN  string `json:"isbn"`
	Title string `json:"title"`
}

type BookRepository interface {
	List() []Book
}

type MemoryBookDao struct {
	books []Book
}

func NewMemoryBookDao() *MemoryBookDao {
	return &MemoryBookDao{books: []Book{{ISBN: "978-0134190440", Title: "The Go Programming Language"}}}
}

func (d *MemoryBookDao) List() []Book {
	return append([]Book(nil), d.books...)
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

type StartupCheckRunner struct {
	Service  *BookService `autowire:""`
	Required bool         `value:"${bookman.seed.required:=true}"`
}

func (r *StartupCheckRunner) Run(ctx context.Context) error {
	if r.Required && len(r.Service.ListBooks()) == 0 {
		return errors.New("book seed data is empty")
	}
	log.Printf("startup check passed, books=%d", len(r.Service.ListBooks()))
	return nil
}

type BookStatsJob struct {
	Service *BookService `autowire:""`
}

func (j *BookStatsJob) Run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Print("book stats job stopped")
			return
		case <-ticker.C:
			log.Printf("book count=%d", len(j.Service.ListBooks()))
		}
	}
}

type JobRunner struct {
	Job *BookStatsJob `autowire:""`
}

func (r *JobRunner) Run(ctx context.Context) error {
	go r.Job.Run(ctx)
	return nil
}

type BookController struct {
	Service *BookService `autowire:""`
}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(c.Service.ListBooks())
}

func init() {
	gs.Provide(NewMemoryBookDao).Export(gs.As[BookRepository]())
	gs.Provide(NewBookService)
	gs.Provide(&StartupCheckRunner{}).Export(gs.As[gs.Runner]())
	gs.Provide(&BookStatsJob{})
	gs.Provide(&JobRunner{}).Export(gs.As[gs.Runner]())
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
