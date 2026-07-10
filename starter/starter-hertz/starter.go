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

		// Register a Hertz-backed HTTP server
		// when the application provides a *server.Hertz.
		r.Provide(NewSimpleHertzServer).
			Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[*server.Hertz]())
		return nil
	})
}

// SimpleHertzServer adapts server.Hertz to the Go-Spring server lifecycle.
// The application is expected to build and configure the *server.Hertz
// (host/port, routes, middlewares) itself; this adapter only drives its
// start/stop according to the Go-Spring readiness signal.
type SimpleHertzServer struct {
	h *server.Hertz
}

// NewSimpleHertzServer wraps a user-provided *server.Hertz.
func NewSimpleHertzServer(h *server.Hertz) *SimpleHertzServer {
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
