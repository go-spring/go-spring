// Package middleware provides tRPC filters (unary) for the tRPC server.
package middleware

import "context"

// UnaryInterceptor is the prototype for tRPC unary filters.
type UnaryInterceptor func(next UnaryHandler) UnaryHandler

// UnaryHandler processes a unary request and returns a response.
type UnaryHandler func(ctx context.Context, req any) (any, error)

// ChainUnary applies multiple unary filters in order (outermost first).
func ChainUnary(h UnaryHandler, mws ...UnaryInterceptor) UnaryHandler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

type RecoveryConfig struct {
	Msg string `value:"${msg:=recovery}"`
}

// Recovery returns a filter that recovers from panics during a call.
func Recovery(config RecoveryConfig) UnaryInterceptor {
	return func(next UnaryHandler) UnaryHandler {
		return func(ctx context.Context, req any) (resp any, err error) {
			// TODO: defer/recover and map the panic to a trpc error.
			return next(ctx, req)
		}
	}
}

type TraceConfig struct {
	Msg string `value:"${msg:=trace}"`
}

// Trace returns a filter that records a trace span for each call.
func Trace(config TraceConfig) UnaryInterceptor {
	return func(next UnaryHandler) UnaryHandler {
		return func(ctx context.Context, req any) (any, error) {
			// TODO: start/close a trace span around next.
			return next(ctx, req)
		}
	}
}

type MetricConfig struct {
	Msg string `value:"${msg:=metric}"`
}

// Metric returns a filter that records metrics for each call.
func Metric(config MetricConfig) UnaryInterceptor {
	return func(next UnaryHandler) UnaryHandler {
		return func(ctx context.Context, req any) (any, error) {
			// TODO: measure latency / count around next.
			return next(ctx, req)
		}
	}
}
