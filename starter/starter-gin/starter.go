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
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

var ginTag = log.RegisterAppTag("gin", "starter")

func init() {
	gin.SetMode(gin.ReleaseMode)

	gs.Provide(
		NewSimpleGinServer,
		gs.IndexArg(1, gs.TagArg("?")),
		gs.IndexArg(2, gs.TagArg("${spring.gin.server}")),
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.gin.server.addr"))
}

// RouterRegister registers routes and middleware onto the framework-owned
// *gin.Engine. This function type keeps SimpleGinServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
//
// When the built-in middleware set is enabled (middleware.enabled, default
// true), the starter installs it before the register runs so it wraps every
// application route: Observe (Recovery + Tracing + Metrics + AccessLog),
// RequestID, and the opt-in CORS/Gzip/SecureHeaders. When an application
// disables the set, the register owns the entire chain - including Recovery -
// and may call ApplyMiddlewares (or the individual constructors) itself to place
// the built-ins wherever it likes. Mount only routes and app-specific middleware
// here.
type RouterRegister func(e *gin.Engine)

// EngineMiddleware installs middleware onto the framework-owned *gin.Engine at
// server startup, before the built-in middleware set runs. Provide one as a bean
// to run application middleware on the OUTSIDE of the built-in chain - e.g. an
// auth or trace-context middleware that must run before RequestID and Observe -
// without disabling the defaults:
//
//	gs.Provide(func() StarterGin.EngineMiddleware {
//	    return func(e *gin.Engine) { e.Use(myAuthMiddleware) }
//	})
//
// It is injected as a single nullable bean (the "?" autowire tag in the starter's
// init), so it is nil when no EngineMiddleware bean is provided - no config or
// ceremony required, and providing more than one fails the container with an
// ambiguity error (a single hook is enough: install e.Use(...) as many times as
// needed inside it). It runs only when the built-in set is enabled; in manual
// mode (middleware.enabled=false) the application owns the whole chain via its
// RouterRegister and calls ApplyMiddlewares directly, so the hook is irrelevant.
type EngineMiddleware func(e *gin.Engine)

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
//
// outer is the application-supplied EngineMiddleware hook (nullable - nil when
// none is provided); it runs before the built-in set so app middleware sits on
// the outside of the chain. cfg is bound from ${spring.gin.server}.
func NewSimpleGinServer(register RouterRegister, outer EngineMiddleware, cfg Config) (*SimpleGinServer, error) {
	e := gin.New()

	// Run the application-supplied outer hook first, so it wraps the built-in
	// chain (it ends up outermost - before RequestID). nil when the app
	// provides no EngineMiddleware bean.
	if outer != nil {
		outer(e)
	}

	if cfg.Middleware.Enabled {
		if err := ApplyMiddlewares(e, cfg); err != nil {
			return nil, err
		}
	}

	// Register the optional health endpoint before application routes so it is
	// always available and cannot be shadowed by a wildcard route.
	if cfg.Health.Enabled {
		e.GET(cfg.Health.Path, func(c *gin.Context) {
			c.String(http.StatusOK, "ok")
		})
	}

	register(e)

	addr := cfg.Address
	tlsEnabled := cfg.TLS.Enabled
	log.Debugf(context.Background(), ginTag, "gin server created addr=%s tls=%v readTimeout=%s writeTimeout=%s idleTimeout=%s",
		addr, tlsEnabled, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)

	return &SimpleGinServer{
		svr: &http.Server{
			Addr:        addr,
			Handler:     e,
			ReadTimeout: cfg.ReadTimeout,
			// No separate header-timeout config: read-header time reuses
			// readTimeout (also bounds slowloris-style slow-header attacks).
			ReadHeaderTimeout: cfg.ReadTimeout,
			WriteTimeout:      cfg.WriteTimeout,
			IdleTimeout:       cfg.IdleTimeout,
		},
		tls:      tlsEnabled,
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
	log.Infof(ctx, ginTag, "gin server starting on %s (tls=%v)", s.svr.Addr, s.tls)

	if s.tls {
		err = s.svr.ServeTLS(ln, s.certFile, s.keyFile)
	} else {
		err = s.svr.Serve(ln)
	}

	if errors.Is(err, http.ErrServerClosed) {
		log.Debugf(ctx, ginTag, "gin server stopped on %s", s.svr.Addr)
		return nil
	}
	if err != nil {
		log.Errorf(ctx, ginTag, "gin server failed on %s: %v", s.svr.Addr, err)
	}
	return errutil.Explain(err, "failed to serve on %s", s.svr.Addr)
}

// Stop gracefully shuts the HTTP server down, allowing in-flight requests to
// complete.
func (s *SimpleGinServer) Stop() error {
	log.Infof(context.Background(), ginTag, "gin server shutting down on %s", s.svr.Addr)
	return s.svr.Shutdown(context.Background())
}
