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

// Package logger forwards kitex' framework logs (klog) into go-spring's log
// module, so an application only configures one logging pipeline. The bridge
// self-installs via init(): the main StarterKitex package blank-imports this
// package, so importing the starter redirects kitex' default stderr sink into
// the same sink the application already configures for go-spring's log.
package logger

import (
	"context"
	"fmt"
	"io"

	"github.com/cloudwego/kitex/pkg/klog"
	"go-spring.org/log"
)

// kitexTag tags every forwarded line as an RPC log under "rpc.kitex" so it
// can be filtered or routed to a dedicated logger later without touching the
// framework wiring.
var kitexTag = log.RegisterRPCTag("kitex", "")

// loggerAdapter implements kitex' klog.FullLogger by forwarding every framework
// log line into go-spring's log module. Importing this starter puts kitex under
// go-spring's management: its server wiring, etcd resolver events, transport
// errors and handler klog calls all flow through the same pipeline the
// application already configures for go-spring's log, instead of kitex' default
// stderr sink.
//
// The CtxXxx methods carry a context.Context, so go-spring's FieldsFromContext
// hook can extract trace_id/span_id off the incoming request and correlate this
// log line with its span. The plain Logger/FormatLogger paths have no context
// and fall back to context.Background(), losing trace correlation on that path
// only; the caller (file:line) recorded by go-spring points into this bridge
// rather than the real emit site, which is inherent to any adapter of this
// shape.
//
// NOTE: the bridge only redirects "who writes the log". The application must
// still configure a go-spring log sink (e.g. a root FileLogger under
// ${logging.logger}); without one, forwarded kitex logs land on go-spring's
// default console rather than the app's own output.
type loggerAdapter struct{}

// init installs the bridge before any kitex component captures klog's default
// logger, so every log line for the lifetime of the process is redirected into
// go-spring's log module.
func init() {
	klog.SetLogger(&loggerAdapter{})
}

// toGSLevel maps klog levels onto go-spring's levels. kitex has a Notice level
// that go-spring does not model; it is folded into Info to preserve the payload
// without inventing a new go-spring level for a single framework's quirk.
func toGSLevel(level klog.Level) log.Level {
	switch level {
	case klog.LevelTrace:
		return log.TraceLevel
	case klog.LevelDebug:
		return log.DebugLevel
	case klog.LevelInfo:
		return log.InfoLevel
	case klog.LevelNotice:
		return log.InfoLevel
	case klog.LevelWarn:
		return log.WarnLevel
	case klog.LevelError:
		return log.ErrorLevel
	case klog.LevelFatal:
		return log.FatalLevel
	default:
		return log.InfoLevel
	}
}

// record is the single sink both the non-format and format paths flow through
// so level mapping, tagging and skip-depth stay in one place. skip=3 steps out
// of record + the caller klog method + go-spring's Record so the recorded
// caller site points at the framework code that invoked klog rather than at
// this bridge (best-effort - see the type doc).
func (l *loggerAdapter) record(ctx context.Context, level klog.Level, msg string) {
	log.Record(ctx, toGSLevel(level), kitexTag, 3, log.Msg(msg))
}

// Logger - v...any methods. kitex passes no context on this signature, so
// context.Background() is used and trace correlation is unavailable on this
// path. Callers inside kitex are almost always framework internals (startup,
// transport wiring), not per-request code.

func (l *loggerAdapter) Trace(v ...any) {
	l.record(context.Background(), klog.LevelTrace, fmt.Sprint(v...))
}
func (l *loggerAdapter) Debug(v ...any) {
	l.record(context.Background(), klog.LevelDebug, fmt.Sprint(v...))
}
func (l *loggerAdapter) Info(v ...any) {
	l.record(context.Background(), klog.LevelInfo, fmt.Sprint(v...))
}
func (l *loggerAdapter) Notice(v ...any) {
	l.record(context.Background(), klog.LevelNotice, fmt.Sprint(v...))
}
func (l *loggerAdapter) Warn(v ...any) {
	l.record(context.Background(), klog.LevelWarn, fmt.Sprint(v...))
}
func (l *loggerAdapter) Error(v ...any) {
	l.record(context.Background(), klog.LevelError, fmt.Sprint(v...))
}
func (l *loggerAdapter) Fatal(v ...any) {
	l.record(context.Background(), klog.LevelFatal, fmt.Sprint(v...))
}

// FormatLogger - printf-style, same background-context caveat as Logger.

func (l *loggerAdapter) Tracef(format string, v ...any) {
	l.record(context.Background(), klog.LevelTrace, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Debugf(format string, v ...any) {
	l.record(context.Background(), klog.LevelDebug, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Infof(format string, v ...any) {
	l.record(context.Background(), klog.LevelInfo, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Noticef(format string, v ...any) {
	l.record(context.Background(), klog.LevelNotice, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Warnf(format string, v ...any) {
	l.record(context.Background(), klog.LevelWarn, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Errorf(format string, v ...any) {
	l.record(context.Background(), klog.LevelError, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) Fatalf(format string, v ...any) {
	l.record(context.Background(), klog.LevelFatal, fmt.Sprintf(format, v...))
}

// CtxLogger - the whole point of the bridge: pass the caller's ctx through so
// go-spring's FieldsFromContext hook can lift trace_id/span_id off the incoming
// request and correlate this log line with its span. Handlers should prefer
// these methods (klog.CtxInfof etc.) over the non-ctx variants above.

func (l *loggerAdapter) CtxTracef(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelTrace, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxDebugf(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelDebug, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxInfof(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelInfo, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxNoticef(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelNotice, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxWarnf(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelWarn, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxErrorf(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelError, fmt.Sprintf(format, v...))
}
func (l *loggerAdapter) CtxFatalf(ctx context.Context, format string, v ...any) {
	l.record(ctx, klog.LevelFatal, fmt.Sprintf(format, v...))
}

// Control - deliberate no-ops. Once the bridge is installed, go-spring owns
// both the level filtering (via each Logger's level config) and the output
// sink (via FileLogger/ConsoleLogger appenders), so kitex' per-logger knobs
// have no meaning here. Leaving them as stubs keeps the klog.FullLogger
// contract satisfied without pretending to honor calls we would silently
// discard.
func (l *loggerAdapter) SetLevel(klog.Level) {}
func (l *loggerAdapter) SetOutput(io.Writer) {}
