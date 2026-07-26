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

// Package logger forwards tRPC's framework logs into go-spring's log module,
// so an application only configures one logging pipeline. The bridge
// self-installs via init(): the main StarterTrpc package blank-imports this
// package, so importing the starter redirects tRPC's default zap console sink
// into the same sink the application already configures for go-spring's log.
package logger

import (
	"context"
	"fmt"

	"go-spring.org/log"
	trpclog "trpc.group/trpc-go/trpc-go/log"
)

// trpcTag tags every forwarded line as an RPC log under "rpc.trpc" so it can
// be filtered or routed later without touching the framework wiring.
var trpcTag = log.RegisterRPCTag("trpc", "")

// loggerAdapter implements tRPC's log.Logger by forwarding every framework log
// line into go-spring's log module. Importing this package puts tRPC under
// go-spring's management: its server wiring, transport errors and handler log
// calls all flow through the same pipeline the application already configures
// for go-spring's log, instead of tRPC's default zap console sink.
//
// tRPC's base Logger interface carries no context.Context (per-request trace
// correlation is done through log.WithContext binding fields, not through these
// methods), so forwarded lines use context.Background() and cannot be tagged
// with an incoming trace_id/span_id on this path - the same limitation the
// non-ctx paths of the other framework bridges have.
//
// NOTE: the bridge only redirects "who writes the log". The application must
// still configure a go-spring log sink (e.g. a root FileLogger under
// ${logging.logger}); without one, forwarded tRPC logs land on go-spring's
// default console rather than the app's own output.
type loggerAdapter struct{}

// init installs the bridge before any tRPC component captures the default
// logger, so every log line for the lifetime of the process is redirected.
func init() {
	trpclog.SetLogger(&loggerAdapter{})
}

// record is the single sink all paths flow through so tagging and skip-depth
// stay in one place. skip=3 steps out of record + the caller log method +
// go-spring's Record so the recorded caller site points at the framework code
// that invoked the logger rather than at this bridge.
func (l *loggerAdapter) record(level log.Level, msg string) {
	log.Record(context.Background(), level, trpcTag, 3, log.Msg(msg))
}

func (l *loggerAdapter) Trace(args ...interface{}) { l.record(log.TraceLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Debug(args ...interface{}) { l.record(log.DebugLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Info(args ...interface{})  { l.record(log.InfoLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Warn(args ...interface{})  { l.record(log.WarnLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Error(args ...interface{}) { l.record(log.ErrorLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Fatal(args ...interface{}) { l.record(log.FatalLevel, fmt.Sprint(args...)) }

func (l *loggerAdapter) Tracef(format string, args ...interface{}) {
	l.record(log.TraceLevel, fmt.Sprintf(format, args...))
}
func (l *loggerAdapter) Debugf(format string, args ...interface{}) {
	l.record(log.DebugLevel, fmt.Sprintf(format, args...))
}
func (l *loggerAdapter) Infof(format string, args ...interface{}) {
	l.record(log.InfoLevel, fmt.Sprintf(format, args...))
}
func (l *loggerAdapter) Warnf(format string, args ...interface{}) {
	l.record(log.WarnLevel, fmt.Sprintf(format, args...))
}
func (l *loggerAdapter) Errorf(format string, args ...interface{}) {
	l.record(log.ErrorLevel, fmt.Sprintf(format, args...))
}
func (l *loggerAdapter) Fatalf(format string, args ...interface{}) {
	l.record(log.FatalLevel, fmt.Sprintf(format, args...))
}

// Sync is a no-op: go-spring owns flushing through its own appenders.
func (l *loggerAdapter) Sync() error { return nil }

// SetLevel/GetLevel are no-ops/stubs. Once the bridge is installed, go-spring
// owns level filtering via each logger's config, so tRPC's per-output knobs have
// no meaning here.
func (l *loggerAdapter) SetLevel(output string, level trpclog.Level) {}
func (l *loggerAdapter) GetLevel(output string) trpclog.Level        { return trpclog.LevelInfo }

// With drops the fields and returns the same logger: go-spring's log module
// carries its own structured fields, and tRPC's base Logger cannot thread them
// into go-spring's Record on this path.
func (l *loggerAdapter) With(fields ...trpclog.Field) trpclog.Logger { return l }
