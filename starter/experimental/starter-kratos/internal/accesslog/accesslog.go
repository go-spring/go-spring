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

// Package accesslog writes one log line per served RPC.
//
// It is a sibling of internal/logger, not part of it: that package bridges
// whatever kratos itself logs, which is a different thing from one line per
// call. The observation is otherwise the library's (kratos/v2's tracing and
// metrics middlewares emit the span and the metrics), so this is the only
// per-call signal go-spring contributes — without it the RPC family would have
// no log line to join a failing metric to.
package accesslog

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/transport"
	"go-spring.org/log"
)

// rpcSystem is the value the RPC family's rpc.system label carries for this
// backend, so the line joins the metrics kratos/v2 emits.
const rpcSystem = "kratos"

// tag is the static log tag for the kratos access log. RegisterAppTag registers
// or retrieves, so importing both the grpc and the http subpackage is safe.
var tag = log.RegisterAppTag("kratos", "access")

// Server returns the middleware that writes one access-log line per call. Its
// identity keys are the RPC family's shared ones (rpc.system / rpc.method /
// status), so selecting a failing method on a dashboard lands on the lines that
// explain it. duration_ms and error are the line's own payload.
func Server() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			start := time.Now()
			rsp, err := handler(ctx, req)
			dur := time.Since(start)

			status := statusOf(err)
			fields := []log.Field{
				log.String("rpc.system", rpcSystem),
				log.String("rpc.method", operation(ctx)),
				log.String("status", status),
				log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
			}
			if err != nil {
				log.Warn(ctx, tag, append(fields, log.Err(err))...)
				return rsp, err
			}
			log.Info(ctx, tag, fields...)
			return rsp, nil
		}
	}
}

// operation names the call the way kratos does — the transport reports it, so
// the log line matches the span name kratos/v2's tracing middleware sets.
func operation(ctx context.Context) string {
	if tr, ok := transport.FromServerContext(ctx); ok {
		return tr.Operation()
	}
	return ""
}

// statusOf names the outcome the way the family's status label and log field
// expect — the same two words the other RPC backends use.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
