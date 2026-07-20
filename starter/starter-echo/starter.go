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

package StarterEcho

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/spring/starter"
)

func init() {
	enableSimpleEchoServer := gs.OnProperty("spring.echo.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleEchoServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register an Echo-backed HTTP server when the application provides a
		// RouterRegister bean. The starter owns the *echo.Echo and its
		// http.Server (config from ${spring.echo.server}); the app only supplies
		// the route/middleware registration.
		r.Provide(
			NewSimpleEchoServer,
			gs.IndexArg(1, gs.TagArg("${spring.echo.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[RouterRegister]())
		return nil
	})
}

// RouterRegister registers routes and middleware onto the framework-owned
// *echo.Echo. This function type keeps SimpleEchoServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
type RouterRegister func(e *echo.Echo)

// HealthConfig exposes an optional liveness/readiness endpoint served by the
// starter. It is disabled by default so applications opt in explicitly.
type HealthConfig struct {
	Enabled bool   `value:"${enabled:=false}"`
	Path    string `value:"${path:=/healthz}"`
}

// Config defines Echo server configuration, bound from ${spring.echo.server}.
// The embedded gs.SimpleHttpServerConfig carries the address and read/header/
// write/idle timeouts; the extra fields add HTTPS, a request-body size limit,
// and an optional health endpoint without touching the spring core struct.
type Config struct {
	gs.SimpleHttpServerConfig
	MaxBodySize int64             `value:"${maxBodySize:=0}"`
	TLS         starter.TLSConfig `value:"${tls}"`
	Health      HealthConfig      `value:"${health}"`
}

// SimpleEchoServer adapts an Echo engine to the Go-Spring server lifecycle. It
// owns a standard http.Server so it can serve either plaintext HTTP or, when
// TLS is configured, HTTPS.
type SimpleEchoServer struct {
	svr      *http.Server
	tls      bool
	certFile string
	keyFile  string
}

// NewSimpleEchoServer builds an *echo.Echo with framework defaults, applies the
// registered RouterRegister, and wraps it in an HTTP server configured from
// ${spring.echo.server}.
func NewSimpleEchoServer(register RouterRegister, cfg Config) *SimpleEchoServer {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())

	// Register the optional health endpoint before application routes so it is
	// always available and cannot be shadowed by a wildcard route.
	if cfg.Health.Enabled {
		e.GET(cfg.Health.Path, func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})
	}

	register(e)

	var handler http.Handler = e
	if cfg.MaxBodySize > 0 {
		handler = http.MaxBytesHandler(handler, cfg.MaxBodySize)
	}

	return &SimpleEchoServer{
		svr: &http.Server{
			Addr:              cfg.Address,
			Handler:           handler,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.HeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		tls:      cfg.TLS.Enabled,
		certFile: cfg.TLS.CertFile,
		keyFile:  cfg.TLS.KeyFile,
	}
}

// Run binds the listener immediately and starts serving after Go-Spring signals
// readiness. When TLS is enabled it serves HTTPS from the configured cert/key.
func (s *SimpleEchoServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.svr.Addr)
	}
	<-sig.TriggerAndWait()
	if s.tls {
		err = s.svr.ServeTLS(ln, s.certFile, s.keyFile)
	} else {
		err = s.svr.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}

// Stop gracefully shuts the HTTP server down, allowing in-flight requests to
// complete.
func (s *SimpleEchoServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
