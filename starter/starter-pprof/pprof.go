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

package StarterPProf

import (
	"net/http"
	"net/http/pprof"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/httpauth"
)

// Config configures the dedicated pprof HTTP server. pprof endpoints expose
// sensitive runtime internals (goroutine stacks, heap, CPU profiles), so they
// must not be reachable unauthenticated off-host. The default address binds to
// all interfaces (:9981) for operational convenience; when that is used without
// any authentication the constructor logs a warning. Lock it down by binding to
// loopback (127.0.0.1:9981) or by setting Token / Username+Password.
type Config struct {
	// Address is the listen address. It defaults to ":9981" (all interfaces);
	// use 127.0.0.1:9981 to restrict access to the local host.
	Address string `value:"${addr:=:9981}"`

	// Token, when set, requires each request to present it as an
	// "Authorization: Bearer <token>" header. Takes precedence over
	// Username/Password.
	Token string `value:"${token:=}"`

	// Username and Password, when both set, require HTTP Basic authentication.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
}

// SimplePProfServer is a simple HTTP server that exposes pprof endpoints.
type SimplePProfServer struct {
	*gs.SimpleHttpServer
}

// NewSimplePProfServer creates a new SimplePProfServer from the config. It
// registers the standard pprof handlers, wraps them with the configured
// authentication guard, and warns when the endpoints would be reachable
// off-host without any authentication.
func NewSimplePProfServer(ctx *gs.ContextProvider, c Config) *SimplePProfServer {
	mux := http.NewServeMux()

	// Register pprof handlers
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	guard := httpauth.Guard{Token: c.Token, Username: c.Username, Password: c.Password}
	if !guard.Enabled() && !httpauth.IsLoopback(c.Address) {
		log.Warnf(ctx.Context, log.TagAppDef,
			"pprof server listening on %q without authentication; set ${spring.pprof.token} or ${spring.pprof.username}/${spring.pprof.password}",
			c.Address)
	}

	cfg := gs.SimpleHttpServerConfig{Address: c.Address}
	return &SimplePProfServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: guard.Wrap(mux)}, cfg),
	}
}
