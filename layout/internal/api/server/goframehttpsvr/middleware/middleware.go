// Package middleware provides HTTP middlewares for the goframe HTTP server.
package middleware

import (
	"fmt"
	"net/http"
)

// Middleware is the prototype for goframe HTTP middlewares. It is expressed as
// an http.Handler wrapper so the scaffold stays dependency-free; a real
// integration would translate this to ghttp.HandlerFunc via a small adapter
// once the goframe dep is added.
type Middleware func(next http.Handler) http.Handler

// Chain applies multiple middlewares to an http.Handler in order (outermost first).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type RecoveryConfig struct {
	Msg string `value:"${msg:=recovery}"`
}

// Recovery returns a middleware that recovers from panics during a request.
func Recovery(config RecoveryConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(config.Msg)
			defer func() {
				if err := recover(); err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type TraceConfig struct {
	Msg string `value:"${msg:=trace}"`
}

// Trace returns a middleware that records a trace span for each request.
func Trace(config TraceConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(config.Msg)
			next.ServeHTTP(w, r)
		})
	}
}

type MetricConfig struct {
	Msg string `value:"${msg:=metric}"`
}

// Metric returns a middleware that records metrics for each request.
func Metric(config MetricConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(config.Msg)
			next.ServeHTTP(w, r)
		})
	}
}
