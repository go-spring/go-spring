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
	"github.com/labstack/echo/v4"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleEchoServer := gs.OnProperty("spring.echo.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleEchoServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register an Echo-backed HTTP server
		// when the application provides an echo.Echo.
		r.Provide(
			NewSimpleEchoServer,
			gs.IndexArg(1, gs.TagArg("${spring.echo.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[*echo.Echo]())
		return nil
	})
}

// SimpleEchoServer adapts echo.Echo to the Go-Spring server lifecycle.
type SimpleEchoServer struct {
	*gs.SimpleHttpServer
}

// NewSimpleEchoServer creates an Echo HTTP server using ${spring.echo.server} configuration.
func NewSimpleEchoServer(e *echo.Echo, cfg gs.SimpleHttpServerConfig) *SimpleEchoServer {
	return &SimpleEchoServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: e}, cfg),
	}
}
