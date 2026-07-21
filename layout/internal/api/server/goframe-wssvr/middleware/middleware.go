// Package middleware provides HTTP middleware for the goframe WebSocket upgrade handshake.
package middleware

import (
	"fmt"
	"net/http"
)

// Middleware defines the prototype for handshake middlewares.
// A Middleware wraps an http.Handler and returns a new http.Handler.
// It runs on the HTTP request that precedes the WebSocket upgrade, so it
// cannot see individual frames once the connection is established.
type Middleware func(next http.Handler) http.Handler

// Chain applies multiple middlewares to an http.Handler in order.
// Middlewares are applied from first to last (outermost to innermost).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type RecoveryConfig struct {
	Msg string `value:"${msg:=recovery}"`
}

// Recovery returns a middleware that recovers from panics raised during the
// upgrade handshake.
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

// Trace returns a middleware that logs a trace message for each handshake.
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

// Metric returns a middleware that logs a metric message for each handshake.
func Metric(config MetricConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(config.Msg)
			next.ServeHTTP(w, r)
		})
	}
}
