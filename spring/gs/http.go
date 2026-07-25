/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package gs

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var httpServerTag = log.RegisterAppTag("http", "server")

func init() {
	// Register a module for HTTP server.
	enableSimpleHttpServer := OnProperty("spring.http.server.enabled").
		HavingValue("true").MatchIfMissing()
	Module(enableSimpleHttpServer, func(r BeanProvider, p flatten.Storage) error {

		// Register the default HTTP multiplexer (http.ServeMux) as a bean
		// only when no user-defined *HttpServeMux is present.
		r.Provide(&HttpServeMux{http.DefaultServeMux}).
			Condition(OnMissingBean[*HttpServeMux]())

		// Provide a new SimpleHttpServer instance with
		// HTTP handler injection and configuration binding.
		r.Provide(
			NewSimpleHttpServer,
			IndexArg(1, TagArg("${spring.http.server}")),
		).Export(As[Server]())

		return nil
	})
}

// HttpServeMux is a lightweight wrapper around an http.Handler,
// allowing the default http.ServeMux or a custom handler
// to be injected into the HTTP server.
type HttpServeMux struct {
	http.Handler
}

// SimpleHttpServerConfig holds configuration for SimpleHttpServer.
type SimpleHttpServerConfig struct {
	// Address specifies the TCP address the server listens on.
	// Example: ":9090" (listen on all interfaces, port 9090).
	Address string `value:"${addr:=:9090}"`

	// ReadTimeout is the maximum duration for reading the entire
	// HTTP request, including the body.
	ReadTimeout time.Duration `value:"${readTimeout:=5s}"`

	// HeaderTimeout is the maximum duration for reading request headers.
	HeaderTimeout time.Duration `value:"${headerTimeout:=1s}"`

	// WriteTimeout is the maximum duration before timing out
	// an HTTP response write.
	WriteTimeout time.Duration `value:"${writeTimeout:=5s}"`

	// IdleTimeout is the maximum time to wait for the next request
	// when keep-alive connections are enabled.
	IdleTimeout time.Duration `value:"${idleTimeout:=60s}"`
}

// SimpleHttpServer wraps a standard http.Server to integrate it
// into the Go-Spring application lifecycle.
type SimpleHttpServer struct {
	svr *http.Server // Underlying HTTP server instance.
}

// NewSimpleHttpServer constructs a new SimpleHttpServer using
// the provided HTTP handler and configuration.
func NewSimpleHttpServer(h *HttpServeMux, cfg SimpleHttpServerConfig) *SimpleHttpServer {
	var handler http.Handler
	if h != nil {
		handler = h.Handler
	}
	log.Debugf(context.Background(), httpServerTag, "creating HTTP server: addr=%s readTimeout=%s writeTimeout=%s idleTimeout=%s",
		cfg.Address, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)
	return &SimpleHttpServer{svr: &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadTimeout:       cfg.ReadTimeout,
		ReadHeaderTimeout: cfg.HeaderTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}}
}

// Run starts the HTTP server and blocks until it is stopped.
// It listens on the configured address immediately, but waits
// for the given ReadySignal before accepting traffic.
func (s *SimpleHttpServer) Run(ctx context.Context, sig ReadySignal) error {
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		log.Errorf(ctx, httpServerTag, "failed to listen on %s: %v", s.svr.Addr, err)
		return errutil.Explain(err, "failed to listen on %s", s.svr.Addr)
	}
	log.Infof(ctx, httpServerTag, "HTTP server listening on %s", s.svr.Addr)
	<-sig.TriggerAndWait()
	err = s.svr.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		log.Infof(ctx, httpServerTag, "HTTP server closed gracefully")
		return nil
	}
	log.Errorf(ctx, httpServerTag, "HTTP server serve error: %v", err)
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}

// Stop gracefully stops the HTTP server, allowing in-flight requests
// to complete.
func (s *SimpleHttpServer) Stop() error {
	log.Debugf(context.Background(), httpServerTag, "stopping HTTP server on %s", s.svr.Addr)
	return s.svr.Shutdown(context.Background())
}
