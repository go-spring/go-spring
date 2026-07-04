package main

import (
	"fmt"
	"net/http"

	"go-spring.org/spring/gs"
)

type EchoHandler struct {
	AppName string `value:"${bookman.app.name:=BookMan Pro}"`
	Enabled bool   `value:"${bookman.feature.echo:=true}"`
	Message string `value:"${bookman.echo.message:=BookMan Pro is running}"`
	Prefix  string `value:"${bookman.echo.prefix:=local}"`
}

func (h *EchoHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !h.Enabled {
		http.Error(w, "echo disabled", http.StatusNotFound)
		return
	}
	_, _ = fmt.Fprintf(w, "%s: %s", h.Prefix, h.Message)
}

func init() {
	gs.Provide(&EchoHandler{})
	gs.Provide(func(h *EchoHandler) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("/echo", h.ServeHTTP)
		return &gs.HttpServeMux{Handler: mux}
	})
}

func main() {
	gs.Run()
}
