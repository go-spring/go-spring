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
	"context"
	"net/http"
	"strings"
)

// RequestContext abstracts an HTTP request lifecycle context.
// It provides unified access to the request, response writer.
type RequestContext interface {
	// Request returns the underlying *http.Request.
	Request() *http.Request

	// ResponseWriter returns the underlying http.ResponseWriter.
	ResponseWriter() http.ResponseWriter

	// PathValue returns the value of the named path parameter.
	PathValue(name string) string
}

type ctxKeyType struct{}

var ctxKey ctxKeyType

// GetRequestContext retrieves the RequestContext stored in ctx.
// It returns nil if no RequestContext is present.
func GetRequestContext(ctx context.Context) RequestContext {
	rc, _ := ctx.Value(&ctxKey).(RequestContext)
	return rc
}

// WithRequestContext returns a new context derived from ctx
// that carries the given RequestContext.
func WithRequestContext(ctx context.Context, c RequestContext) context.Context {
	return context.WithValue(ctx, &ctxKey, c)
}

// SimpleContext is a minimal RequestContext implementation
// backed directly by http.Request and http.ResponseWriter.
type SimpleContext struct {
	r *http.Request
	w http.ResponseWriter
}

// NewRequestContext defines a factory function used to create
// a RequestContext from an HTTP request and response writer.
type NewRequestContext func(r *http.Request, w http.ResponseWriter) RequestContext

// NewSimpleContext creates a SimpleContext instance and returns it
// as a RequestContext.
func NewSimpleContext(r *http.Request, w http.ResponseWriter) RequestContext {
	return &SimpleContext{r: r, w: w}
}

// Request returns the underlying *http.Request.
func (c *SimpleContext) Request() *http.Request {
	return c.r
}

// ResponseWriter returns the underlying http.ResponseWriter.
func (c *SimpleContext) ResponseWriter() http.ResponseWriter {
	return c.w
}

// PathValue returns the value of the named path parameter.
func (c *SimpleContext) PathValue(name string) string {
	return c.r.PathValue(name)
}

// Router describes a single HTTP route definition,
// including method, URL pattern, and handler.
type Router struct {
	Method  string
	Pattern string
	Handler http.HandlerFunc
}

// Server defines the minimal contract for an HTTP server
// capable of registering routes.
type Server interface {
	Route(r Router)
}

// SimpleServer is a lightweight HTTP server implementation
// based on http.Server and http.ServeMux.
type SimpleServer struct {
	*http.Server
	mux *http.ServeMux
}

// NewSimpleServer creates a SimpleServer bound to the given address.
func NewSimpleServer(addr string) *SimpleServer {
	mux := http.NewServeMux()
	svr := &http.Server{
		Addr:    addr,
		Handler: mux,
	}
	return &SimpleServer{Server: svr, mux: mux}
}

// Route registers a route with method-based pattern matching.
// The pattern format follows Go 1.22+ ServeMux conventions,
// for example: "GET /users/{id}".
func (s *SimpleServer) Route(r Router) {
	pattern := strings.TrimSpace(r.Method + " " + r.Pattern)
	s.mux.HandleFunc(pattern, r.Handler)
}
