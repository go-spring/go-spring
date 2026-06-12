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

package ginsvr

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-spring/gs-http-gen/lib/pathidl"
	"github.com/go-spring/stdlib/httpsvr"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

type ctxKeyType struct{}

var ctxKey ctxKeyType

// getContext retrieves the *gin.Context from a standard context.Context.
// It assumes the value has already been set and will panic if missing.
func getContext(ctx context.Context) *gin.Context {
	return ctx.Value(&ctxKey).(*gin.Context)
}

// setContext stores the *gin.Context into a standard context.Context
// and returns the derived context.
func setContext(ctx context.Context, c *gin.Context) context.Context {
	return context.WithValue(ctx, &ctxKey, c)
}

// GinRequestContext adapts *gin.Context to the httpsvr.RequestContext interface.
type GinRequestContext struct {
	*gin.Context
}

// NewGinRequestContext creates a new httpsvr.RequestContext
// from an http.Request and http.ResponseWriter.
func NewGinRequestContext(r *http.Request, w http.ResponseWriter) httpsvr.RequestContext {
	c := getContext(r.Context())
	return &GinRequestContext{c}
}

// Request returns the underlying *http.Request.
func (c *GinRequestContext) Request() *http.Request {
	return c.Context.Request
}

// ResponseWriter returns the underlying http.ResponseWriter.
func (c *GinRequestContext) ResponseWriter() http.ResponseWriter {
	return c.Context.Writer
}

// PathValue returns the value of a path parameter
// extracted from the Gin route.
func (c *GinRequestContext) PathValue(name string) string {
	return c.Context.Param(name)
}

// GinServer wraps an http.Server and a Gin engine.
// It acts as an adapter between httpsvr and Gin.
type GinServer struct {
	*http.Server
	engine *gin.Engine
}

// NewGinServer creates and initializes a GinServer
// listening on the specified address.
func NewGinServer(addr string) *GinServer {
	engine := gin.New()
	svr := &http.Server{
		Addr:    addr,
		Handler: engine.Handler(),
	}
	return &GinServer{Server: svr, engine: engine}
}

// ToGinPath converts a pathidl-style pattern into
// a Gin-compatible routing path.
//
// Examples:
//
//	"/users/{id}"      -> "/users/:id"
//	"/files/{*path}"   -> "/files/*path"
func ToGinPath(pattern string) string {
	path, _ := pathidl.Parse(pattern)
	var sb strings.Builder
	for _, s := range path {
		sb.WriteString("/")
		switch s.Type {
		case pathidl.Static:
			sb.WriteString(s.Value)
		case pathidl.Param:
			sb.WriteString(":")
			sb.WriteString(s.Value)
		case pathidl.Wildcard:
			sb.WriteString("*")
			sb.WriteString(s.Value)
		}
	}
	return sb.String()
}

// HandleFunc registers an HTTP route using Gin
// based on the provided httpsvr.Router definition.
func (s *GinServer) HandleFunc(r httpsvr.Router) {
	s.engine.Handle(r.Method, ToGinPath(r.Pattern), func(c *gin.Context) {
		ctx := setContext(c.Request.Context(), c)
		r.Handler(c.Writer, c.Request.WithContext(ctx))
	})
}
