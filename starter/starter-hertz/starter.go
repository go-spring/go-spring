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

package StarterHertz

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleHertzServer := gs.OnProperty("spring.hertz.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleHertzServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register a Hertz-backed HTTP server when the application provides a
		// RouterRegister bean. The starter owns the *server.Hertz and its
		// listener (address from ${spring.hertz.server}); the app only supplies
		// the route/middleware registration.
		r.Provide(
			NewSimpleHertzServer,
			gs.IndexArg(1, gs.TagArg("${spring.hertz.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[RouterRegister]())
		return nil
	})
}

// RouterRegister registers routes and middleware onto the framework-owned
// *server.Hertz. This function type keeps SimpleHertzServer route-agnostic: the
// starter creates and configures the engine, while each application supplies
// its own register bean to wire handlers.
//
// Built-in cross-cutting middlewares (Recovery, RequestID, AccessLog, and the
// opt-in CORS/Gzip/SecureHeaders) are installed by the starter before the
// register runs, so they wrap every application route. Mount only routes and
// app-specific middleware here.
type RouterRegister func(h *server.Hertz)

// SimpleHertzServer adapts a *server.Hertz to the Go-Spring server lifecycle.
// The starter builds and configures the engine (address, timeouts, TLS, routes
// via the RouterRegister); this adapter only drives its start/stop according to
// the Go-Spring readiness signal.
type SimpleHertzServer struct {
	h *server.Hertz
}

// NewSimpleHertzServer builds a *server.Hertz listening on the configured
// address, applies timeout/body/TLS options and the built-in middlewares, and
// applies the registered RouterRegister. It uses server.New (not server.Default)
// so Recovery is configurable via the middleware block. It returns an error
// when a built-in middleware (notably CORS) is misconfigured, so the server
// fails fast at startup instead of panicking on the first request.
func NewSimpleHertzServer(register RouterRegister, cfg Config) (*SimpleHertzServer, error) {
	opts := []config.Option{
		server.WithHostPorts(cfg.Addr),
		server.WithReadTimeout(cfg.ReadTimeout),
		server.WithWriteTimeout(cfg.WriteTimeout),
		server.WithIdleTimeout(cfg.IdleTimeout),
	}
	if cfg.MaxBodySize > 0 {
		opts = append(opts, server.WithMaxRequestBodySize(cfg.MaxBodySize))
	}
	if cfg.TLS.Enabled {
		tlsCfg, err := cfg.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "hertz: build TLS")
		}
		opts = append(opts, server.WithTLS(tlsCfg))
	}

	h := server.New(opts...)

	if err := applyMiddlewares(h, cfg); err != nil {
		return nil, err
	}

	// Register the optional health endpoint before application routes so it is
	// always available and cannot be shadowed by a wildcard route.
	if cfg.Health.Enabled {
		h.GET(cfg.Health.Path, func(ctx context.Context, c *app.RequestContext) {
			c.String(200, "ok")
		})
	}

	register(h)
	return &SimpleHertzServer{h: h}, nil
}

// Run starts the Hertz engine after Go-Spring signals readiness.
func (s *SimpleHertzServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	return s.h.Run()
}

// Stop gracefully shuts the Hertz engine down.
func (s *SimpleHertzServer) Stop() error {
	return s.h.Shutdown(context.Background())
}
