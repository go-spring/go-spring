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

// Package logger forwards dubbo-go's framework logs into go-spring's log
// module, so an application only configures one logging pipeline. The bridge
// self-installs via init(): the main StarterDubbo package blank-imports this
// package, so importing the starter redirects dubbo-go's two layered logger
// facades - dubbo.apache.org/dubbo-go/v3/logger and
// github.com/dubbogo/gost/log/logger - into the same sink the application
// already configures for go-spring's log.
package logger

import (
	"context"
	"fmt"

	dubbologger "dubbo.apache.org/dubbo-go/v3/logger"
	gostlogger "github.com/dubbogo/gost/log/logger"
	"go-spring.org/log"
)

// dubboTag tags every forwarded line as an RPC log under "rpc.dubbo" so it
// can be filtered or routed to a dedicated logger later without touching
// framework wiring. The tag is shared across all dubbo-go protocols (triple /
// dubbo / jsonrpc / rest) so downstream filters do not have to distinguish
// sub-protocols.
var dubboTag = log.RegisterRPCTag("dubbo", "")

// loggerAdapter implements dubbo-go's Logger interface by forwarding every
// framework log line into go-spring's log module. Importing this starter puts
// dubbo-go under go-spring's management, so its internal logs flow through the
// same pipeline the application already configures for go-spring's log -
// otherwise dubbo-go's zap default sink prints to stdout, out of band with the
// app's own logs.
//
// Dubbo-go has TWO layered logger facades that share an identical 10-method
// interface (Debug/Info/Warn/Error/Fatal + f variants):
//   - dubbo.apache.org/dubbo-go/v3/logger - used by the high-level dubbo-go
//     stack (config loader, protocol, registry).
//   - github.com/dubbogo/gost/log/logger - the underlying facade used by getty
//     (the classic-Dubbo transport) and much of the internal library code.
//
// Setting only the top-level facade leaves getty and low-level modules writing
// to their own default sink, so we install the SAME bridge instance under
// BOTH SetLogger entrypoints in init() below. The two Logger interfaces are
// structurally identical, so one type satisfies both.
//
// NOTE: the bridge only redirects "who writes the log". The application must
// still configure a go-spring log sink (e.g. a root FileLogger under
// ${logging.logger}); without one, forwarded dubbo-go logs land on go-spring's
// default console rather than the app's own output.
//
// Trade-off: dubbo-go's Logger methods carry no context.Context, so
// trace-id propagation via go-spring's FieldsFromContext hook is not
// available on this path, and the caller (file:line) recorded by go-spring
// points into this bridge rather than the real emit site. We accept the
// caller imprecision.
type loggerAdapter struct{}

// init installs the bridge before any dubbo-go component reads
// logger.GetLogger(). Both facades keep a package-level variable that is
// captured by callers at first use, so replacing them here - during Go
// package init, well before gs.Run() starts the server - is enough to
// redirect every log line for the lifetime of the process.
func init() {
	b := &loggerAdapter{}
	dubbologger.SetLogger(b)
	gostlogger.SetLogger(b)
}

// record is the single sink used by all ten interface methods. skip=3 walks
// past record + the interface method + go-spring's own record() to try to
// keep the caller pointing at the framework emit site.
func (l *loggerAdapter) record(level log.Level, msg string) {
	log.Record(context.Background(), level, dubboTag, 3, log.Msg(msg))
}

// The variadic (sugared) methods concatenate args the same way zap's
// SugaredLogger does under the hood (fmt.Sprint), so the message shape
// matches what dubbo-go callers already expect.
func (l *loggerAdapter) Debug(args ...any) { l.record(log.DebugLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Info(args ...any)  { l.record(log.InfoLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Warn(args ...any)  { l.record(log.WarnLevel, fmt.Sprint(args...)) }
func (l *loggerAdapter) Error(args ...any) { l.record(log.ErrorLevel, fmt.Sprint(args...)) }

// Fatal maps to go-spring's FatalLevel. We deliberately do NOT call os.Exit
// here: go-spring's log module owns the fatal semantics.
func (l *loggerAdapter) Fatal(args ...any) { l.record(log.FatalLevel, fmt.Sprint(args...)) }

// The f-variants use the caller's format string directly.
func (l *loggerAdapter) Debugf(template string, args ...any) {
	l.record(log.DebugLevel, fmt.Sprintf(template, args...))
}
func (l *loggerAdapter) Infof(template string, args ...any) {
	l.record(log.InfoLevel, fmt.Sprintf(template, args...))
}
func (l *loggerAdapter) Warnf(template string, args ...any) {
	l.record(log.WarnLevel, fmt.Sprintf(template, args...))
}
func (l *loggerAdapter) Errorf(template string, args ...any) {
	l.record(log.ErrorLevel, fmt.Sprintf(template, args...))
}
func (l *loggerAdapter) Fatalf(template string, args ...any) {
	l.record(log.FatalLevel, fmt.Sprintf(template, args...))
}
