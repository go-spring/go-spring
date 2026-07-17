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
	"fmt"
	"io"

	"github.com/cloudwego/kitex/pkg/klog"
	"go-spring.org/log"
)

// gsBridgeLogger implements kitex' klog.FullLogger by forwarding every framework
// log line into go-spring's log module. Together with the go-kratos pilot, this
// is part of a wider goal: let each contrib framework's internal logs flow
// through go-spring's log so users only configure one logging pipeline (here the
// root FileLogger in conf/app.properties, which ships to Loki). kitex' server
// wiring, etcd resolver events, transport errors and handler klog calls thus
// land in provider.log as JSON, next to the business logs.
//
// The CtxXxx methods carry a context.Context, so go-spring's FieldsFromContext
// hook can extract trace_id/span_id off the incoming request and correlate this
// log line with its span. The plain Logger/FormatLogger paths have no context
// and fall back to context.Background(), losing trace correlation on that path
// only; the caller (file:line) recorded by go-spring points into this bridge
// rather than the real emit site, which is inherent to any adapter of this
// shape.
type gsBridgeLogger struct {
	tag *log.Tag
}

// newGSBridgeLogger builds the bridge, tagging every forwarded line as an RPC
// log under "rpc.kitex" so it can be filtered or routed to a dedicated logger
// later without touching the framework wiring. It returns klog.FullLogger so
// klog.SetLogger accepts it directly (FullLogger is the composite of Logger,
// FormatLogger, CtxLogger and Control).
func newGSBridgeLogger() klog.FullLogger {
	return &gsBridgeLogger{tag: log.RegisterRPCTag("kitex", "")}
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
// this bridge (best-effort — see the type doc).
func (l *gsBridgeLogger) record(ctx context.Context, level klog.Level, msg string) {
	log.Record(ctx, toGSLevel(level), l.tag, 3, log.Msg(msg))
}

// Logger — v...any methods. kitex passes no context on this signature, so
// context.Background() is used and trace correlation is unavailable on this
// path. Callers inside kitex are almost always framework internals (startup,
// transport wiring), not per-request code.

func (l *gsBridgeLogger) Trace(v ...any)  { l.record(context.Background(), klog.LevelTrace, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Debug(v ...any)  { l.record(context.Background(), klog.LevelDebug, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Info(v ...any)   { l.record(context.Background(), klog.LevelInfo, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Notice(v ...any) { l.record(context.Background(), klog.LevelNotice, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Warn(v ...any)   { l.record(context.Background(), klog.LevelWarn, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Error(v ...any)  { l.record(context.Background(), klog.LevelError, fmt.Sprint(v...)) }
func (l *gsBridgeLogger) Fatal(v ...any)  { l.record(context.Background(), klog.LevelFatal, fmt.Sprint(v...)) }

// FormatLogger — printf-style, same background-context caveat as Logger.

func (l *gsBridgeLogger) Tracef(format string, v ...any)  { l.record(context.Background(), klog.LevelTrace, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Debugf(format string, v ...any)  { l.record(context.Background(), klog.LevelDebug, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Infof(format string, v ...any)   { l.record(context.Background(), klog.LevelInfo, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Noticef(format string, v ...any) { l.record(context.Background(), klog.LevelNotice, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Warnf(format string, v ...any)   { l.record(context.Background(), klog.LevelWarn, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Errorf(format string, v ...any)  { l.record(context.Background(), klog.LevelError, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) Fatalf(format string, v ...any)  { l.record(context.Background(), klog.LevelFatal, fmt.Sprintf(format, v...)) }

// CtxLogger — the whole point of the bridge: pass the caller's ctx through so
// go-spring's FieldsFromContext hook can lift trace_id/span_id off the incoming
// request and correlate this log line with its span. Handlers should prefer
// these methods (klog.CtxInfof etc.) over the non-ctx variants above.

func (l *gsBridgeLogger) CtxTracef(ctx context.Context, format string, v ...any)  { l.record(ctx, klog.LevelTrace, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxDebugf(ctx context.Context, format string, v ...any)  { l.record(ctx, klog.LevelDebug, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxInfof(ctx context.Context, format string, v ...any)   { l.record(ctx, klog.LevelInfo, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxNoticef(ctx context.Context, format string, v ...any) { l.record(ctx, klog.LevelNotice, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxWarnf(ctx context.Context, format string, v ...any)   { l.record(ctx, klog.LevelWarn, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxErrorf(ctx context.Context, format string, v ...any)  { l.record(ctx, klog.LevelError, fmt.Sprintf(format, v...)) }
func (l *gsBridgeLogger) CtxFatalf(ctx context.Context, format string, v ...any)  { l.record(ctx, klog.LevelFatal, fmt.Sprintf(format, v...)) }

// Control — deliberate no-ops. Once the bridge is installed, go-spring owns
// both the level filtering (via each Logger's level config) and the output
// sink (via FileLogger/ConsoleLogger appenders), so kitex' per-logger knobs
// have no meaning here. Leaving them as stubs keeps the klog.FullLogger
// contract satisfied without pretending to honor calls we would silently
// discard.
func (l *gsBridgeLogger) SetLevel(klog.Level) {}
func (l *gsBridgeLogger) SetOutput(io.Writer) {}
