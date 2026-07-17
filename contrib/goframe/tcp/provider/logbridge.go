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

package main

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"go-spring.org/log"
)

// gsGoFrameTag tags every forwarded line as an RPC log under "rpc.goframe" so
// the go-spring sink can route or filter goframe internals separately from
// business logs, matching the go-kratos bridge pattern.
var gsGoFrameTag = log.RegisterRPCTag("goframe", "")

// gsGoFrameLogHandler is a glog.Handler that forwards every framework log line
// into go-spring's log module. It is the goframe counterpart of the kratos log
// bridge: with it installed, gtcp lifecycle events, gsvc registration errors
// and any g.Log().* call all land in the same JSON FileLogger pipeline as the
// business logs (which ships to Loki), so users only configure one pipeline.
//
// The handler intentionally does NOT call in.Next(ctx). glog appends its
// doFinalPrint handler after any user-installed default handler (see
// glog_logger.go around SetDefaultHandler / handlers slice), so skipping Next
// terminates the chain there and fully suppresses glog's own stdout/file
// output. This is why we picked SetDefaultHandler over the coarser SetWriter
// fallback: Handler gives us ctx (so go-spring's FieldsFromContext / trace-id
// propagation works) AND structured access to Level / Content / Values, while
// still letting us cut off the default sink.
func gsGoFrameLogHandler(ctx context.Context, in *glog.HandlerInput) {
	// in.Content is glog's already-formatted message body (Sprintf'd by the
	// caller); trim the trailing newline glog adds so go-spring's own
	// formatter is not double-spaced.
	msg := strings.TrimRight(in.Content, "\n")

	fields := make([]log.Field, 0, 2)
	fields = append(fields, log.Msg(msg))

	// in.Values holds the un-formatted args a caller passed alongside the
	// message (e.g. g.Log().Info(ctx, "greet", user, id) → Values = [user,id]).
	// glog itself has no key/value contract on this slice — the elements are
	// arbitrary, positional, and often not paired. So rather than pretend
	// they are k/v pairs, attach the whole slice under a single "values"
	// field. Consumers who need structure should log a formatted message.
	if len(in.Values) > 0 {
		fields = append(fields, log.Any("values", in.Values))
	}

	// skip=2 walks past this handler and go-spring's own record() frame, but
	// the caller printed by the sink is still inherently unreliable across
	// glog's internal doPrint layers — same trade-off the kratos bridge notes.
	log.Record(ctx, toGSGoFrameLevel(in.Level), gsGoFrameTag, 2, fields...)
}

// toGSGoFrameLevel maps glog's bitmask level constants onto go-spring's levels.
// glog has two extra "notice" and "critical" rungs that go-spring lacks: NOTI
// folds into Info (it is just an emphasised Info in glog) and CRIT folds into
// Fatal (glog's own CRIT is its highest emitted level below the never-emitted
// PANI/FATA prefix markers — see glog_logger_level.go). PANI/FATA are only
// used by glog for prefix formatting on Panic/Fatal calls, but we map them
// too so a stray forwarded event still gets the right severity.
func toGSGoFrameLevel(level int) log.Level {
	switch level {
	case glog.LEVEL_DEBU:
		return log.DebugLevel
	case glog.LEVEL_INFO, glog.LEVEL_NOTI:
		return log.InfoLevel
	case glog.LEVEL_WARN:
		return log.WarnLevel
	case glog.LEVEL_ERRO:
		return log.ErrorLevel
	case glog.LEVEL_CRIT, glog.LEVEL_FATA:
		return log.FatalLevel
	case glog.LEVEL_PANI:
		return log.PanicLevel
	default:
		return log.InfoLevel
	}
}

// installGoFrameLogBridge routes every glog log line through go-spring's log
// module. It is called once at server construction (see server.go), before any
// gtcp handler runs, so the very first framework log — including gsvc
// registration and any startup error — flows through the bridge.
//
// We install on both surfaces:
//
//   - glog.SetDefaultHandler covers freshly-created *glog.Logger instances
//     (any package that calls glog.New() will inherit our handler).
//   - g.Log().SetHandlers overrides the default for goframe's process-wide
//     singleton logger, because per-logger Handlers take precedence over the
//     package default (see glog_logger.go: `if len(l.config.Handlers) > 0`).
//     Without this second call, anything that goes through g.Log() would
//     still hit glog's own writer.
func installGoFrameLogBridge() {
	glog.SetDefaultHandler(gsGoFrameLogHandler)
	g.Log().SetHandlers(gsGoFrameLogHandler)
}
