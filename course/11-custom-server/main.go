package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/go-spring/spring-core/gs"
)

type Book struct {
	ISBN  string `json:"isbn"`
	Title string `json:"title"`
}

type BookController struct{}

func (c *BookController) List(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode([]Book{{ISBN: "978-0134190440", Title: "The Go Programming Language"}})
}

type EchoServerConfig struct {
	Addr string `value:"${bookman.echo-server.addr:=:10090}"`
}

type EchoServer struct {
	addr string
	ln   net.Listener
}

func NewEchoServer(c EchoServerConfig) *EchoServer {
	return &EchoServer{addr: c.Addr}
}

func (s *EchoServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	s.ln = ln

	<-sig.TriggerAndWait()

	for {
		conn, err := ln.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				return err
			}
		}
		go handleConn(conn)
	}
}

func (s *EchoServer) Stop() error {
	if s.ln == nil {
		return nil
	}
	err := s.ln.Close()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func handleConn(conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "quit" {
			return
		}
		_, _ = conn.Write([]byte("echo: " + line + "\n"))
	}
}

func init() {
	gs.Provide(&BookController{})
	gs.Provide(func(c *BookController) *gs.HttpServeMux {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /books", c.List)
		return &gs.HttpServeMux{Handler: mux}
	})
	gs.Provide(NewEchoServer).
		Condition(gs.OnProperty("bookman.echo-server.enabled").HavingValue("true").MatchIfMissing()).
		Export(gs.As[gs.Server]())
}

func main() {
	gs.Run()
}
