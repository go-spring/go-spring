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
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/config"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/spring/starter"
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
// starter creates and configures the engine, while each application supplies its
// own register bean to wire handlers.
type RouterRegister func(h *server.Hertz)

// HealthConfig exposes an optional liveness/readiness endpoint served by the
// starter. It is disabled by default so applications opt in explicitly.
type HealthConfig struct {
	Enabled bool   `value:"${enabled:=false}"`
	Path    string `value:"${path:=/healthz}"`
}

// Config defines Hertz server configuration, bound from ${spring.hertz.server}.
// Unlike gin/echo, Hertz owns its own listener, so the address and the
// read/write/idle timeouts are passed to the engine via server options rather
// than a standard http.Server. Field naming mirrors gs.SimpleHttpServerConfig.
type Config struct {
	Addr         string            `value:"${addr:=:8003}"`
	ReadTimeout  time.Duration     `value:"${readTimeout:=5s}"`
	WriteTimeout time.Duration     `value:"${writeTimeout:=5s}"`
	IdleTimeout  time.Duration     `value:"${idleTimeout:=60s}"`
	MaxBodySize  int               `value:"${maxBodySize:=0}"`
	TLS          starter.TLSConfig `value:"${tls}"`
	Health       HealthConfig      `value:"${health}"`
}

// SimpleHertzServer adapts a *server.Hertz to the Go-Spring server lifecycle.
// The starter builds and configures the engine (address, timeouts, TLS, routes
// via the RouterRegister); this adapter only drives its start/stop according to
// the Go-Spring readiness signal.
type SimpleHertzServer struct {
	h *server.Hertz
}

// NewSimpleHertzServer builds a *server.Hertz listening on the configured
// address, applies timeout/body/TLS options, and applies the registered
// RouterRegister.
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

	h := server.Default(opts...)

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
