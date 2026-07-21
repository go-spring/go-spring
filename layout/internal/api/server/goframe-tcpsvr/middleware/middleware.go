// Package middleware provides connection-level middleware for the goframe TCP server.
package middleware

import (
	"context"
	"net"
)

// ConnHandler processes a single accepted TCP connection.
type ConnHandler func(ctx context.Context, conn net.Conn)

// Middleware wraps a ConnHandler and returns a new ConnHandler.
type Middleware func(next ConnHandler) ConnHandler

// Chain applies multiple middlewares to a ConnHandler in order (outermost first).
func Chain(h ConnHandler, mws ...Middleware) ConnHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type RecoveryConfig struct {
	Msg string `value:"${msg:=recovery}"`
}

// Recovery returns a middleware that recovers from panics raised during a
// connection's lifetime so a bad frame does not tear the accept loop down.
func Recovery(config RecoveryConfig) Middleware {
	return func(next ConnHandler) ConnHandler {
		return func(ctx context.Context, conn net.Conn) {
			defer func() {
				if err := recover(); err != nil {
					_ = conn.Close()
				}
			}()
			next(ctx, conn)
		}
	}
}

type TraceConfig struct {
	Msg string `value:"${msg:=trace}"`
}

// Trace returns a middleware that records a trace span for each connection.
func Trace(config TraceConfig) Middleware {
	return func(next ConnHandler) ConnHandler {
		return func(ctx context.Context, conn net.Conn) {
			// TODO: start/close a trace span around next.
			next(ctx, conn)
		}
	}
}

type MetricConfig struct {
	Msg string `value:"${msg:=metric}"`
}

// Metric returns a middleware that records metrics for each connection.
func Metric(config MetricConfig) Middleware {
	return func(next ConnHandler) ConnHandler {
		return func(ctx context.Context, conn net.Conn) {
			// TODO: measure lifetime / count around next.
			next(ctx, conn)
		}
	}
}
