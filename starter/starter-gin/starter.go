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

package StarterGin

import (
	"context"
	"errors"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleGinServer := gs.OnProperty("spring.gin.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleGinServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register a Gin-backed HTTP server when the application provides a
		// RouterRegister bean. The starter owns the *gin.Engine and its
		// http.Server (config from ${spring.gin.server}); the app only supplies
		// the route/middleware registration.
		r.Provide(
			NewSimpleGinServer,
			gs.IndexArg(1, gs.TagArg("${spring.gin.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[RouterRegister]())
		return nil
	})
}

// RouterRegister registers routes and middleware onto the framework-owned
// *gin.Engine. This function type keeps SimpleGinServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
//
// Built-in cross-cutting middlewares (Recovery, RequestID, AccessLog, and the
// opt-in CORS/Gzip/SecureHeaders) are installed by the starter before the
// register runs, so they wrap every application route. Mount only routes and
// app-specific middleware here.
type RouterRegister func(e *gin.Engine)

// SimpleGinServer adapts a Gin engine to the Go-Spring server lifecycle. It
// owns a standard http.Server so it can serve either plaintext HTTP or, when
// TLS is configured, HTTPS.
type SimpleGinServer struct {
	svr      *http.Server
	tls      bool
	certFile string
	keyFile  string
}

// NewSimpleGinServer builds a *gin.Engine with the configured built-in
// middlewares, applies the registered RouterRegister, and wraps it in an HTTP
// server configured from ${spring.gin.server}. It returns an error when a
// built-in middleware (notably CORS) is misconfigured, so the server fails fast
// at startup instead of panicking on the first request.
func NewSimpleGinServer(register RouterRegister, cfg Config) (*SimpleGinServer, error) {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()

	if err := applyMiddlewares(e, cfg); err != nil {
		return nil, err
	}

	// Register the optional health endpoint before application routes so it is
	// always available and cannot be shadowed by a wildcard route.
	if cfg.Health.Enabled {
		e.GET(cfg.Health.Path, func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	register(e)

	return &SimpleGinServer{
		svr: &http.Server{
			Addr:              cfg.Address,
			Handler:           e,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.HeaderTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		tls:      cfg.TLS.Enabled,
		certFile: cfg.TLS.CertFile,
		keyFile:  cfg.TLS.KeyFile,
	}, nil
}

// Run binds the listener immediately and starts serving after Go-Spring signals
// readiness. When TLS is enabled it serves HTTPS from the configured cert/key.
func (s *SimpleGinServer) Run(ctx context.Context, sig gs.ReadySignal) error {
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
func (s *SimpleGinServer) Stop() error {
	return s.svr.Shutdown(context.Background())
}
