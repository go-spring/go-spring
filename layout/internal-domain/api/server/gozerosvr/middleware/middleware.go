// Package middleware provides go-zero (rest) middleware for the go-zero server
// (recovery, trace, metric).
package middleware

import (
	"net/http"
)

// Middleware defines the prototype for go-zero rest middlewares.
// A Middleware wraps an http.Handler and returns a new http.Handler, which
// matches rest.Middleware so the values here can be passed to svr.Use directly.
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

// Recovery returns a middleware that recovers from panics during request handling.
func Recovery(config RecoveryConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: emit config.Msg through the logger and mark the trace span.
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
			// TODO: start/close a trace span using config.Msg around next.
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
			// TODO: measure latency / count using config.Msg around next.
			next.ServeHTTP(w, r)
		})
	}
}
