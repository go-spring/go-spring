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

	"github.com/cloudwego/hertz/pkg/app/server"
	"go-spring.org/spring/gs"
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
// starter creates and configures the engine, while each application supplies its
// own register bean to wire handlers.
type RouterRegister func(h *server.Hertz)

// Config defines Hertz server configuration, bound from ${spring.hertz.server}.
// Unlike gin/echo, Hertz owns its own listener, so the address is passed to the
// engine via WithHostPorts rather than a standard http.Server.
type Config struct {
	Addr string `value:"${addr:=:8003}"`
}

// SimpleHertzServer adapts a *server.Hertz to the Go-Spring server lifecycle.
// The starter builds and configures the engine (address from config, routes via
// the RouterRegister); this adapter only drives its start/stop according to the
// Go-Spring readiness signal.
type SimpleHertzServer struct {
	h *server.Hertz
}

// NewSimpleHertzServer builds a *server.Hertz listening on the configured
// address and applies the registered RouterRegister.
func NewSimpleHertzServer(register RouterRegister, cfg Config) *SimpleHertzServer {
	h := server.Default(server.WithHostPorts(cfg.Addr))
	register(h)
	return &SimpleHertzServer{h: h}
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
