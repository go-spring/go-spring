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
	"crypto/tls"
	"errors"
	"net"
	"net/http"

	"github.com/labstack/echo/v4"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	gs.Provide(
		NewSimpleEchoServer,
		gs.IndexArg(1, gs.TagArg("?")), // nullable EngineMiddleware outer hook
		gs.IndexArg(2, gs.TagArg("${spring.echo.server}")),
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.echo.server.addr"))
}

// RouterRegister registers routes and middleware onto the framework-owned
// *echo.Echo. This function type keeps SimpleEchoServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
//
// Built-in cross-cutting middlewares (Recovery, RequestID, AccessLog, and the
// opt-in CORS/Gzip/SecureHeaders) are installed by the starter before the
// register runs, so they wrap every application route. Mount only routes and
// app-specific middleware here.
type RouterRegister func(e *echo.Echo)

// EngineMiddleware installs middleware onto the framework-owned *echo.Echo at the
// outermost position — before every built-in (LoadTest/Recovery/RequestID/...),
// the same outer hook gin offers. Supply it as a bean:
//
//	gs.Provide(func() StarterEcho.EngineMiddleware {
//		return func(e *echo.Echo) { e.Use(myCompanyMiddleware) }
//	})
//
// The hook runs before applyMiddlewares, so middleware you add here ends up
// outermost on the chain. It is a single nullable "?" bean: absent when the app
// provides none, with no config or enabled knob.
type EngineMiddleware func(e *echo.Echo)

// SimpleEchoServer adapts an Echo engine to the Go-Spring server lifecycle. It
// owns a standard http.Server so it can serve either plaintext HTTP or, when
// TLS is configured, HTTPS.
type SimpleEchoServer struct {
	svr     *http.Server
	tls     bool
	tlsConf *tls.Config
}

// NewSimpleEchoServer builds an *echo.Echo with the configured built-in
// middlewares, applies the registered RouterRegister, and wraps it in an HTTP
// server configured from ${spring.echo.server}. outer is the application-supplied
// EngineMiddleware hook (nil when none is provided); it runs before the built-in
// chain so middleware it installs ends up outermost.
func NewSimpleEchoServer(register RouterRegister, outer EngineMiddleware, cfg Config) (*SimpleEchoServer, error) {
	e := echo.New()
	e.HideBanner = true

	// Run the application-supplied outer hook first, so it wraps the built-in
	// chain (it ends up outermost — before LoadTest). nil when the app provides
	// no EngineMiddleware bean.
	if outer != nil {
		outer(e)
	}

	if err := applyMiddlewares(e, cfg); err != nil {
		return nil, err
	}

	// Register the optional health endpoint before application routes so it is
	// always available and cannot be shadowed by a wildcard route.
	if cfg.Health.Enabled {
		e.GET(cfg.Health.Path, func(c echo.Context) error {
			return c.String(http.StatusOK, "ok")
		})
	}

	register(e)

	addr := cfg.Address
	tlsEnabled := cfg.TLS.Enabled
	var tlsConf *tls.Config
	if tlsEnabled {
		// BuildServer applies the full ${spring.echo.server.tls.*} block with
		// server semantics, same as starter-grpc: cert-file/key-file is the
		// server pair, and ca-file enables mTLS (RequireAndVerifyClientCert).
		var err error
		tlsConf, err = cfg.TLS.BuildServer()
		if err != nil {
			return nil, errutil.Explain(err, "echo: build TLS")
		}
	}
	log.Debugf(context.Background(), log.TagAppDef, "echo server created addr=%s tls=%v readTimeout=%s writeTimeout=%s idleTimeout=%s",
		addr, tlsEnabled, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)

	return &SimpleEchoServer{
		svr: &http.Server{
			Addr:              addr,
			Handler:           e,
			ReadTimeout:       cfg.ReadTimeout,
			ReadHeaderTimeout: cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		tls:     tlsEnabled,
		tlsConf: tlsConf,
	}, nil
}

// Run binds the listener immediately and starts serving after Go-Spring signals
// readiness. When TLS is enabled it serves HTTPS via tls.NewListener with the
// prebuilt server config (ca-file makes it require client certificates).
func (s *SimpleEchoServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	ln, err := net.Listen("tcp", s.svr.Addr)
	if err != nil {
		return errutil.Explain(err, "failed to listen on %s", s.svr.Addr)
	}
	<-sig.TriggerAndWait()
	log.Infof(ctx, log.TagAppDef, "echo server starting on %s (tls=%v)", s.svr.Addr, s.tls)
	if s.tls {
		err = s.svr.Serve(tls.NewListener(ln, s.tlsConf))
	} else {
		err = s.svr.Serve(ln)
	}
	if errors.Is(err, http.ErrServerClosed) {
		log.Debugf(ctx, log.TagAppDef, "echo server stopped on %s", s.svr.Addr)
		return nil
	}
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "echo server failed on %s: %v", s.svr.Addr, err)
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}

// Stop gracefully shuts the HTTP server down with the given context, allowing
// in-flight requests to complete. The shutdown context is propagated to
// http.Server.Shutdown.
func (s *SimpleEchoServer) Stop(ctx context.Context) error {
	log.Infof(ctx, log.TagAppDef, "echo server shutting down on %s", s.svr.Addr)
	return s.svr.Shutdown(ctx)
}
