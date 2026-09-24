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
// whatever kitex itself logs, which is a different thing from one line per call.
// The span and the metrics come from kitex-contrib/obs-opentelemetry, so this is
// the only per-call signal go-spring contributes — without it the RPC family
// would have no log line to join a failing metric to.
package accesslog

import (
	"context"
	"time"

	"github.com/cloudwego/kitex/pkg/endpoint"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"go-spring.org/log"
)

// rpcSystem is the value the RPC family's rpc.system label carries for this
// backend, so the line joins the metrics and spans the tracing suite emits.
const rpcSystem = "kitex"

// tag is the static log tag for the kitex access log.
var tag = log.RegisterAppTag("kitex", "access")

// Server returns the server middleware that writes one access-log line per
// call. Its identity keys are the RPC family's shared ones (rpc.system /
// rpc.method / status), so selecting a failing method on a dashboard lands on
// the lines that explain it. duration_ms and error are the line's own payload.
func Server() endpoint.Middleware {
	return func(next endpoint.Endpoint) endpoint.Endpoint {
		return func(ctx context.Context, req, resp any) error {
			start := time.Now()
			err := next(ctx, req, resp)
			dur := time.Since(start)

			status := statusOf(err)
			fields := []log.Field{
				log.String("rpc.system", rpcSystem),
				log.String("rpc.method", method(ctx)),
				log.String("status", status),
				log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
			}
			if err != nil {
				log.Warn(ctx, tag, append(fields, log.Err(err))...)
				return err
			}
			log.Info(ctx, tag, fields...)
			return nil
		}
	}
}

// method names the call the way kitex does — service/method from the RPC info —
// so the line matches the rpc.method the tracing suite puts on the span.
func method(ctx context.Context) string {
	ri := rpcinfo.GetRPCInfo(ctx)
	if ri == nil || ri.Invocation() == nil {
		return ""
	}
	return ri.Invocation().ServiceName() + "/" + ri.Invocation().MethodName()
}

// statusOf names the outcome the way the family's status label and log field
// expect — the same two words the other RPC backends use.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}
