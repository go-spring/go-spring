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
	"net/http"

	"bookman/internal/app/controller"
	"bookman/internal/idl/http/proto"

	"github.com/go-spring/log"
	"github.com/go-spring/spring-core/gs"
)

var TagHttpAccess = log.RegisterAppTag("http", "access")

func init() {
	// Registers a custom ServeMux to replace the default implementation.
	gs.Provide(NewServeMux)
}

// NewServeMux creates a new HTTP request multiplexer and registers
// routes with access logging middleware.
func NewServeMux(c *controller.Controller) *gs.HttpServeMux {
	mux := http.NewServeMux()
	proto.RegisterRouter(mux, c, Access())

	// Users can customize routes by adding handlers to the mux
	mux.Handle("GET /", http.FileServer(http.Dir("./public")))
	return &gs.HttpServeMux{Handler: mux}
}

// Access is a middleware to log incoming HTTP requests.
func Access() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Infof(r.Context(), TagHttpAccess, "access %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}
