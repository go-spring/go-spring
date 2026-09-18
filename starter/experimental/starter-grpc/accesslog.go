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

package StarterGrpc

import (
	"context"
	"time"

	"go-spring.org/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// rpcSystem is the value the RPC family's rpc.system label carries for this
// backend — the family's shared vocabulary, not a per-file choice. Together
// with rpc.method and status it is one of the three keys every RPC backend
// must emit under the same name, because a query spanning frameworks has
// nothing else to join on. Transport-specific codes (rpc.grpc.status_code,
// rpc.thrift.status_code) are deliberately NOT shared: their value vocabularies
// differ, so one name would mean two things.
const rpcSystem = "grpc"

// accessTag is the static log tag for the gRPC access log.
var accessTag = log.RegisterAppTag("grpc", "access")

// The access log is installed ALWAYS, not behind [ObserverConfig]: it is the
// one signal the family requires of every member, so a config change must not
// be able to remove it. The tracing and metrics interceptors are separately
// toggleable and stay that way — they ride the OTel globals, this does not.
//
// In the interceptor chain it sits just inside tracing (so the line carries the
// span's trace_id) and outside admission, fault injection and recovery (so it
// reports what the caller actually got, including a rejection or a recovered
// panic).

// AccessLogUnaryInterceptor returns the unary interceptor that writes one
// access-log line per call.
func AccessLogUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logCall(ctx, info.FullMethod, statusOf(err), status.Code(err).String(), time.Since(start), err)
		return resp, err
	}
}

// AccessLogStreamInterceptor returns the stream interceptor that writes one
// access-log line per stream.
func AccessLogStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, ss)
		logCall(ss.Context(), info.FullMethod, statusOf(err), status.Code(err).String(), time.Since(start), err)
		return err
	}
}

// statusOf names the outcome the way the family's status label and log field
// expect — the same two words the other RPC backends use, so a dashboard can
// span frameworks. The transport-specific code stays on rpc.grpc.status_code.
func statusOf(err error) string {
	if err != nil {
		return "error"
	}
	return "ok"
}

// logCall writes the per-call access log. Its identity keys are the ones the
// metrics carry (rpc.system / rpc.method / status), so selecting a failing
// method on a dashboard lands on the lines that explain it. code, duration_ms
// and error are the line's own payload.
func logCall(ctx context.Context, method, status, code string, dur time.Duration, err error) {
	fields := []log.Field{
		log.String("rpc.system", rpcSystem),
		log.String("rpc.method", method),
		log.String("status", status),
		log.String("rpc.grpc.status_code", code),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if err != nil {
		log.Warn(ctx, accessTag, append(fields, log.Any("error", err))...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}
