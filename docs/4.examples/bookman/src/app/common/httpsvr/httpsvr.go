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

package httpsvr

import (
	"fmt"
	"log/slog"
	"net/http"

	"bookman/src/app/controller"
	"bookman/src/idl/http/proto"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	// Registers a custom ServeMux to replace the default implementation.
	gs.Provide(
		NewServeMux,
		gs.IndexArg(1, gs.TagArg("access")),
	)
}

// NewServeMux creates a new HTTP request multiplexer and registers
// routes with access logging middleware.
func NewServeMux(c *controller.Controller, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()
	proto.RegisterRouter(mux, c, Access(logger))

	// Users can customize routes by adding handlers to the mux
	mux.Handle("GET /", http.FileServer(http.Dir("./public")))
	return mux
}

// Access is a middleware to log incoming HTTP requests.
func Access(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info(fmt.Sprintf("access %s %s", r.Method, r.URL.Path))
			next.ServeHTTP(w, r)
		})
	}
}
