package main

import (
	"net/http"

	"go-spring.org/spring/gs"
)

func main() {
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("BookMan Pro is running"))
	})

	gs.Run()
}
