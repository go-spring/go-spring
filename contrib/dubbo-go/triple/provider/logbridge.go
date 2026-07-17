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

	dubbologger "dubbo.apache.org/dubbo-go/v3/logger"
	gostlogger "github.com/dubbogo/gost/log/logger"
	"go-spring.org/log"
)

// gsBridgeLogger implements dubbo-go's Logger interface by forwarding every
// framework log line into go-spring's log module. This continues the goal
// piloted for kratos: let each contrib framework's internal logs flow through
// go-spring's log so users only configure one logging pipeline (here the root
// FileLogger in conf/app.properties). Dubbo-go's zap default sink otherwise
// prints to stdout, out of band with the app's own JSON logs.
//
// Dubbo-go has TWO layered logger facades that share an identical 10-method
// interface (Debug/Info/Warn/Error/Fatal + f variants):
//   - dubbo.apache.org/dubbo-go/v3/logger — used by the high-level dubbo-go
//     stack (config loader, protocol, registry).
//   - github.com/dubbogo/gost/log/logger — the underlying facade used by getty
//     (the classic-Dubbo transport) and much of the internal library code.
//
// Setting only the top-level facade leaves getty and low-level modules writing
// to their own default sink, so we install the SAME bridge instance under
// BOTH SetLogger entrypoints in init() below. The two Logger interfaces are
// structurally identical, so one type satisfies both.
//
// Trade-off: dubbo-go's Logger methods carry no context.Context, so
// trace-id propagation via go-spring's FieldsFromContext hook is not
// available on this path, and the caller (file:line) recorded by go-spring
// points into this bridge rather than the real emit site. We accept the
// caller imprecision — same trade-off the kratos bridge makes.
type gsBridgeLogger struct {
	tag *log.Tag
}

// newGSBridgeLogger builds the bridge, tagging every forwarded line as an RPC
// log under "rpc.dubbo" so it can be filtered or routed to a dedicated
// logger later without touching framework wiring. The tag string is shared
// across all four dubbo-go sub-projects (triple / dubbo / jsonrpc / rest) so
// downstream filters do not have to distinguish sub-protocols.
func newGSBridgeLogger() *gsBridgeLogger {
	return &gsBridgeLogger{tag: log.RegisterRPCTag("dubbo", "")}
}

// init installs the bridge before any dubbo-go component reads
// logger.GetLogger(). Both facades keep a package-level variable that is
// captured by callers at first use, so replacing them here — during Go
// package init, well before gs.Run() starts the server — is enough to
// redirect every log line for the lifetime of the process.
func init() {
	b := newGSBridgeLogger()
	dubbologger.SetLogger(b)
	gostlogger.SetLogger(b)
}

// record is the single sink used by all ten interface methods. skip=3 walks
// past record + the interface method + go-spring's own record() to try to
// keep the caller pointing at the framework emit site, matching the kratos
// bridge's skip convention.
func (l *gsBridgeLogger) record(level log.Level, msg string) {
	log.Record(context.Background(), level, l.tag, 3, log.Msg(msg))
}

// The variadic (sugared) methods concatenate args the same way zap's
// SugaredLogger does under the hood (fmt.Sprint), so the message shape
// matches what dubbo-go callers already expect.
func (l *gsBridgeLogger) Debug(args ...any) { l.record(log.DebugLevel, fmt.Sprint(args...)) }
func (l *gsBridgeLogger) Info(args ...any)  { l.record(log.InfoLevel, fmt.Sprint(args...)) }
func (l *gsBridgeLogger) Warn(args ...any)  { l.record(log.WarnLevel, fmt.Sprint(args...)) }
func (l *gsBridgeLogger) Error(args ...any) { l.record(log.ErrorLevel, fmt.Sprint(args...)) }

// Fatal maps to go-spring's FatalLevel. We deliberately do NOT call os.Exit
// here: go-spring's log module owns the fatal semantics, and matching the
// kratos bridge keeps the behaviour consistent across contribs.
func (l *gsBridgeLogger) Fatal(args ...any) { l.record(log.FatalLevel, fmt.Sprint(args...)) }

// The f-variants use the caller's format string directly.
func (l *gsBridgeLogger) Debugf(template string, args ...any) {
	l.record(log.DebugLevel, fmt.Sprintf(template, args...))
}
func (l *gsBridgeLogger) Infof(template string, args ...any) {
	l.record(log.InfoLevel, fmt.Sprintf(template, args...))
}
func (l *gsBridgeLogger) Warnf(template string, args ...any) {
	l.record(log.WarnLevel, fmt.Sprintf(template, args...))
}
func (l *gsBridgeLogger) Errorf(template string, args ...any) {
	l.record(log.ErrorLevel, fmt.Sprintf(template, args...))
}
func (l *gsBridgeLogger) Fatalf(template string, args ...any) {
	l.record(log.FatalLevel, fmt.Sprintf(template, args...))
}
