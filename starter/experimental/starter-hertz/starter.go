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
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/traffic"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	gs.Provide(
		NewSimpleHertzServer,
		gs.IndexArg(1, gs.TagArg("${spring.hertz.server}")),
		// The governance beans are REQUIRED: each is registered by the package that
		// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
		// starter imports — "governance off" is spring.governance.enabled=false, never
		// an absent bean.
		gs.IndexArg(2, gs.TagArg("")),  // *fault.Injector
		gs.IndexArg(3, gs.TagArg("?")), // nullable traffic.Propagator bean
	).Export(gs.As[gs.Server]()).
		Condition(gs.OnProperty("spring.hertz.server.addr"))
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
//
// inj is the fault injector bean gs injects (nil in a standalone call); the
// inbound fault middleware captures it once here rather than resolving per
// request, so a config swap on the bean takes effect without re-resolution.
// prop is the application's load-test convention bean (nil means go-spring's
// default), handed to the inbound LoadTest middleware.
func NewSimpleHertzServer(register RouterRegister, cfg Config, inj *fault.Injector, prop traffic.Propagator) (*SimpleHertzServer, error) {
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
		// BuildServer applies server semantics, same as starter-grpc:
		// cert-file/key-file is the server pair, and ca-file enables mTLS
		// (RequireAndVerifyClientCert). Build() is client-semantics and would
		// ignore CAFile here.
		tlsCfg, err := cfg.TLS.BuildServer()
		if err != nil {
			return nil, errutil.Explain(err, "hertz: build TLS")
		}
		opts = append(opts, server.WithTLS(tlsCfg))
	}

	h := server.New(opts...)

	if err := applyMiddlewares(h, cfg, inj, prop); err != nil {
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

	addr := cfg.Addr
	tlsEnabled := cfg.TLS.Enabled
	log.Debugf(context.Background(), log.TagAppDef, "hertz server created addr=%s tls=%v readTimeout=%s writeTimeout=%s idleTimeout=%s",
		addr, tlsEnabled, cfg.ReadTimeout, cfg.WriteTimeout, cfg.IdleTimeout)

	return &SimpleHertzServer{h: h}, nil
}

// Run starts the Hertz engine after Go-Spring signals readiness.
func (s *SimpleHertzServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	log.Infof(ctx, log.TagAppDef, "hertz server starting")
	if err := s.h.Run(); err != nil {
		log.Errorf(ctx, log.TagAppDef, "hertz server failed: %v", err)
		return err
	}
	return nil
}

// Stop gracefully shuts the Hertz engine down, propagating ctx into the
// engine's context-aware Shutdown so the drain rides the shutdown context.
func (s *SimpleHertzServer) Stop(ctx context.Context) error {
	log.Infof(ctx, log.TagAppDef, "hertz server shutting down")
	return s.h.Shutdown(ctx)
}
