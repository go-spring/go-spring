package main

import (
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BookMan Pro is running"))
	})

	gs.Run()
}
