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

package server

import (
	"context"
	"time"

	v1 "go-spring.org/go-kratos/api/helloworld/v1"
	"go-spring.org/go-kratos/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
	"go-spring.org/spring/gs"
)

// HTTPConfig is the HTTP server configuration, bound from ${spring.kratos.http}.
// The scaffold read these fields from conf.proto's Server.HTTP message; here
// they come from conf/app.properties instead.
type HTTPConfig struct {
	Network string        `value:"${network:=}"`
	Addr    string        `value:"${addr:=0.0.0.0:8000}"`
	Timeout time.Duration `value:"${timeout:=1s}"`
}

// HTTPServer adapts a kratos HTTP transport server to the Go-Spring server
// lifecycle. The scaffold returned the raw *http.Server and let kratos.App call
// Start/Stop; wrapping it as a gs.Server hands that control to gs.Run().
type HTTPServer struct {
	svr *http.Server
}

// NewHTTPServer builds the kratos HTTP server from HTTPConfig and the injected
// GreeterService bean, keeping the scaffold's option wiring intact.
func NewHTTPServer(c HTTPConfig, greeter *service.GreeterService, logger log.Logger) *HTTPServer {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Network != "" {
		opts = append(opts, http.Network(c.Network))
	}
	if c.Addr != "" {
		opts = append(opts, http.Address(c.Addr))
	}
	if c.Timeout != 0 {
		opts = append(opts, http.Timeout(c.Timeout))
	}
	srv := http.NewServer(opts...)
	v1.RegisterGreeterHTTPServer(srv, greeter)
	return &HTTPServer{svr: srv}
}

// Run starts serving once Go-Spring signals readiness. kratos' Start binds the
// listener and blocks until Stop triggers a graceful shutdown, at which point
// it returns nil (it swallows http.ErrServerClosed internally).
func (s *HTTPServer) Run(ctx context.Context, sig gs.ReadySignal) error {
	<-sig.TriggerAndWait()
	return s.svr.Start(ctx)
}

// Stop gracefully shuts down the kratos HTTP server.
func (s *HTTPServer) Stop() error {
	return s.svr.Stop(context.Background())
}
