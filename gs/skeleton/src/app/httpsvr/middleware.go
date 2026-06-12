package httpsvr

import (
	"fmt"
	"net/http"

	"github.com/go-spring/spring-core/gs"
)

func init() {
	gs.Object(&ServerConfig{})
}

// Middleware defines the prototype for HTTP middlewares.
// A Middleware wraps an http.Handler and returns a new http.Handler.
type Middleware func(next http.Handler) http.Handler

// Chain applies multiple middlewares to an http.Handler in order.
// Middlewares are applied from first to last (outermost to innermost).
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// ServerConfig defines the configuration for an HTTP server.
// It contains sub-configs for recovery, tracing, and metrics.
type ServerConfig struct {
	RecoveryConfig RecoveryConfig `value:"${recovery}"`
	TraceConfig    TraceConfig    `value:"${trace}"`
	MetricConfig   MetricConfig   `value:"${metric}"`
}

type RecoveryConfig struct {
	Msg string `value:"${msg:=recovery}"`
}

// Recovery returns a middleware that recovers from panics during request handling.
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

// Trace returns a middleware that logs a trace message for each request.
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

// Metric returns a middleware that logs a metric message for each request.
func Metric(config MetricConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Println(config.Msg)
			next.ServeHTTP(w, r)
		})
	}
}
