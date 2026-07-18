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
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"net/http/pprof"

	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {
	// Registers a SimplePProfServer bean in the IoC container. It is given a
	// distinct name so it coexists with the application's main HTTP server,
	// which also exports gs.Server under the default name.
	gs.Provide(
		NewSimplePProfServer,
		gs.TagArg("${spring.pprof}"),
	).Name("pprofServer").Condition(
		gs.OnProperty("spring.pprof.enabled").HavingValue("true").MatchIfMissing(),
	).Export(gs.As[gs.Server]())
}

// Config configures the dedicated pprof HTTP server. pprof endpoints expose
// sensitive runtime internals (goroutine stacks, heap, CPU profiles), so the
// defaults are deliberately conservative: the server binds to loopback only and
// callers opt into remote exposure explicitly. When a non-loopback Address is
// used, configure Token or Username/Password so the endpoints are not open.
type Config struct {
	// Address is the listen address. It defaults to a loopback-only bind so
	// pprof is not reachable off-host unless the operator opts in.
	Address string `value:"${addr:=127.0.0.1:9981}"`

	// Token, when set, requires each request to present it either as a
	// "Authorization: Bearer <token>" header or a "?token=<token>" query
	// parameter. Takes precedence over Username/Password.
	Token string `value:"${token:=}"`

	// Username and Password, when both set, require HTTP Basic authentication.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
}

// authEnabled reports whether any authentication scheme is configured.
func (c Config) authEnabled() bool {
	return c.Token != "" || (c.Username != "" && c.Password != "")
}

// SimplePProfServer is a simple HTTP server that exposes pprof endpoints.
type SimplePProfServer struct {
	*gs.SimpleHttpServer
}

// NewSimplePProfServer creates a new SimplePProfServer from the config. It
// registers the standard pprof handlers, wraps them with the configured
// authentication guard, and warns when the endpoints would be reachable
// off-host without any authentication.
func NewSimplePProfServer(c Config) *SimplePProfServer {
	mux := http.NewServeMux()

	// Register pprof handlers
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	if !c.authEnabled() && !isLoopback(c.Address) {
		log.Warnf(context.Background(), log.TagAppDef,
			"pprof server listening on %q without authentication; set spring.pprof.token or username/password, or bind to loopback",
			c.Address)
	}

	cfg := gs.SimpleHttpServerConfig{Address: c.Address}
	return &SimplePProfServer{
		SimpleHttpServer: gs.NewSimpleHttpServer(&gs.HttpServeMux{Handler: c.guard(mux)}, cfg),
	}
}

// guard wraps h with the configured authentication scheme. With no scheme
// configured it returns h unchanged (safe because the default bind is
// loopback-only).
func (c Config) guard(h http.Handler) http.Handler {
	if !c.authEnabled() {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.Token != "" {
			if !tokenMatches(r, c.Token) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		} else {
			user, pass, ok := r.BasicAuth()
			if !ok || !constantTimeEqual(user, c.Username) || !constantTimeEqual(pass, c.Password) {
				w.Header().Set("WWW-Authenticate", `Basic realm="pprof"`)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(w, r)
	})
}

// tokenMatches reports whether the request carries the expected token via a
// bearer Authorization header or a "token" query parameter.
func tokenMatches(r *http.Request, want string) bool {
	if got := r.Header.Get("Authorization"); got != "" {
		const prefix = "Bearer "
		if len(got) > len(prefix) && got[:len(prefix)] == prefix &&
			constantTimeEqual(got[len(prefix):], want) {
			return true
		}
	}
	return constantTimeEqual(r.URL.Query().Get("token"), want)
}

// constantTimeEqual compares two strings without leaking their length
// relationship through timing.
func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// isLoopback reports whether addr binds only to a loopback interface. An empty
// or wildcard host (":9981", "0.0.0.0:9981") is treated as non-loopback.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost"
}
