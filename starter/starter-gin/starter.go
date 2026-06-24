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

		// Register a Gin-backed HTTP server
		// when the application provides a gin.Engine.
		r.Provide(
			NewSimpleGinServer,
			gs.IndexArg(1, gs.TagArg("${spring.gin.server}")),
		).Export(gs.As[gs.Server]()).
			Condition(gs.OnBean[*gin.Engine]())
		return nil
	})
}

// SimpleGinServer adapts gin.Engine to the Go-Spring server lifecycle.
type SimpleGinServer struct {
	*gs.SimpleHttpServer
}

// NewSimpleGinServer creates a Gin HTTP server using ${spring.gin.server} configuration.
func NewSimpleGinServer(e *gin.Engine, cfg gs.SimpleHttpServerConfig) *SimpleGinServer {
	return &SimpleGinServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: e}, cfg),
	}
}
