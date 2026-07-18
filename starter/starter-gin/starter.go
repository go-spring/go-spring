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
	"github.com/gin-gonic/gin"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	enableSimpleGinServer := gs.OnProperty("spring.gin.server.enabled").
		HavingValue("true").MatchIfMissing()
	gs.Module(enableSimpleGinServer, func(r gs.BeanProvider, p flatten.Storage) error {

		// Register a Gin-backed HTTP server when the application provides a
		// RouterRegister bean. The starter owns the *gin.Engine and its
		// http.Server (config from ${spring.gin.server}); the app only supplies
		// the route/middleware registration.
		r.Provide(
			NewSimpleGinServer,
			gs.IndexArg(1, gs.TagArg("${spring.gin.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[RouterRegister]())
		return nil
	})
}

// RouterRegister registers routes and middleware onto the framework-owned
// *gin.Engine. This function type keeps SimpleGinServer route-agnostic: the
// starter creates and configures the engine and its HTTP server, while each
// application supplies its own register bean to wire handlers.
type RouterRegister func(e *gin.Engine)

// SimpleGinServer adapts a Gin engine to the Go-Spring server lifecycle.
type SimpleGinServer struct {
	*gs.SimpleHttpServer
}

// NewSimpleGinServer builds a *gin.Engine with framework defaults, applies the
// registered RouterRegister, and wraps it in an HTTP server configured from
// ${spring.gin.server}.
func NewSimpleGinServer(register RouterRegister, cfg gs.SimpleHttpServerConfig) *SimpleGinServer {
	gin.SetMode(gin.ReleaseMode)
	e := gin.New()
	e.Use(gin.Recovery())
	register(e)
	return &SimpleGinServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: e}, cfg),
	}
}
