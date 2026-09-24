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

package StarterThrift

import (
	"context"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"go-spring.org/log"
)

// accessTag is the static log tag for the Thrift access log.
var accessTag = log.RegisterAppTag("thrift", "access")

// AccessLog returns the middleware that writes one access-log line per call.
//
// It is installed ALWAYS, not behind [ObserverConfig]: it is the one signal the
// RPC family requires of every member, so a config change must not be able to
// remove it. The tracing and metrics middleware are separately toggleable and
// stay that way — they ride the OTel globals, this does not.
//
// In the middleware chain it sits inside [Observe] (so the line carries the
// span's trace_id) and outside [Admit] (so a rejected call is logged too).
func AccessLog() thrift.ProcessorMiddleware {
	return func(name string, next thrift.TProcessorFunction) thrift.TProcessorFunction {
		return thrift.WrappedTProcessorFunction{
			Wrapped: func(ctx context.Context, seqID int32, in, out thrift.TProtocol) (bool, thrift.TException) {
				start := time.Now()
				ok, ex := next.Process(ctx, seqID, in, out)
				logCall(ctx, name, statusOf(ex), time.Since(start), ex)
				return ok, ex
			},
		}
	}
}

// statusOf maps a thrift exception to the family's shared success/failure axis —
// the same two words the other RPC backends use, so a dashboard can span
// frameworks.
func statusOf(ex thrift.TException) string {
	if ex != nil {
		return "error"
	}
	return "ok"
}

// logCall writes the per-call access log. Its identity keys are the ones the
// metrics carry (rpc.system / rpc.method / status), so selecting a failing
// method on a dashboard lands on the lines that explain it. duration_ms and
// error are the line's own payload.
func logCall(ctx context.Context, method, status string, dur time.Duration, ex thrift.TException) {
	fields := []log.Field{
		log.String("rpc.system", rpcSystem),
		log.String("rpc.method", method),
		log.String("status", status),
		log.Float("duration_ms", float64(dur.Nanoseconds())/1e6),
	}
	if ex != nil {
		log.Warn(ctx, accessTag, append(fields, log.Err(ex))...)
		return
	}
	log.Info(ctx, accessTag, fields...)
}
