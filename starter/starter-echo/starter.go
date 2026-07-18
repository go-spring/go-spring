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
	"github.com/labstack/echo/v4/middleware"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleEchoServer := gs.OnProperty("spring.echo.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleEchoServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register an Echo-backed HTTP server when the application provides a
		// RouterRegister bean. The starter owns the *echo.Echo and its
		// http.Server (config from ${spring.echo.server}); the app only supplies
		// the route/middleware registration.
		r.Provide(
			NewSimpleEchoServer,
			gs.IndexArg(1, gs.TagArg("${spring.echo.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[RouterRegister]())
		return nil
	})
}

// RouterRegister registers routes and middleware onto the framework-owned
// *echo.Echo. This function type keeps SimpleEchoServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
type RouterRegister func(e *echo.Echo)

// SimpleEchoServer adapts an Echo engine to the Go-Spring server lifecycle.
type SimpleEchoServer struct {
	*gs.SimpleHttpServer
}

// NewSimpleEchoServer builds an *echo.Echo with framework defaults, applies the
// registered RouterRegister, and wraps it in an HTTP server configured from
// ${spring.echo.server}.
func NewSimpleEchoServer(register RouterRegister, cfg gs.SimpleHttpServerConfig) *SimpleEchoServer {
	e := echo.New()
	e.HideBanner = true
	e.Use(middleware.Recover())
	register(e)
	return &SimpleEchoServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: e}, cfg),
	}
}
