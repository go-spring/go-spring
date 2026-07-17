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

// Package logbridge forwards go-zero's framework logs (logx) into go-spring's
// log module, so an application only configures one logging pipeline. It is the
// single shared implementation used by both the rest and zrpc starters — each
// installs it via logx.SetWriter after building its server.
package logbridge

import (
	"context"

	"github.com/zeromicro/go-zero/core/logx"
	"go-spring.org/log"
)

// gsBridgeWriter implements go-zero's logx.Writer by forwarding every framework
// log line into go-spring's log module. This lets go-zero's internal logs
// (transport start/stop, etcd registration, interceptor/middleware errors, stat
// lines, ...) flow through the same go-spring appender as the business logs.
//
// Trade-off: logx.Writer's methods carry no context.Context, so trace-id
// propagation via go-spring's FieldsFromContext hook cannot fire on this path.
// go-zero injects trace/span into `fields` via logx.WithContext, so correlation
// still travels through — we forward every LogField as a structured field. The
// caller (file:line) recorded by go-spring points into this bridge rather than
// the real emit site; we accept that imprecision because logx buries the real
// caller behind several internal frames anyway.
type gsBridgeWriter struct {
	tag *log.Tag
}

// NewWriter builds the bridge, tagging every forwarded line as an RPC log under
// "rpc.gozero" so it can be filtered or routed to a dedicated logger later
// without touching the framework wiring. Install it with logx.SetWriter.
func NewWriter() logx.Writer {
	return &gsBridgeWriter{tag: log.RegisterRPCTag("gozero", "")}
}

// record is the shared sink used by every logx.Writer method. logx methods have
// no context.Context (see type doc), so we always pass context.Background();
// trace/span still arrive as fields. skip=3 walks past record → the calling
// Writer method → go-spring's own record(), landing as close to the emit site
// as this bridge can get.
func (w *gsBridgeWriter) record(level log.Level, v any, fields []logx.LogField) {
	gsFields := make([]log.Field, 0, len(fields)+1)
	gsFields = append(gsFields, log.Msgf("%v", v))
	for _, f := range fields {
		gsFields = append(gsFields, log.Any(f.Key, f.Value))
	}
	log.Record(context.Background(), level, w.tag, 3, gsFields...)
}

// Alert is used by go-zero for alertable events (e.g. custom app-level alerts
// emitted via logx.Alert). Mapped to ErrorLevel; go-spring has no distinct
// alert channel and the caller is expected to already be paging on ErrorLevel.
func (w *gsBridgeWriter) Alert(v any) {
	w.record(log.ErrorLevel, v, nil)
}

// Close is a no-op: go-spring's logger lifecycle is managed by the root
// appender, not by this bridge, so there is nothing to flush here.
func (w *gsBridgeWriter) Close() error {
	return nil
}

func (w *gsBridgeWriter) Debug(v any, fields ...logx.LogField) {
	w.record(log.DebugLevel, v, fields)
}

func (w *gsBridgeWriter) Error(v any, fields ...logx.LogField) {
	w.record(log.ErrorLevel, v, fields)
}

func (w *gsBridgeWriter) Info(v any, fields ...logx.LogField) {
	w.record(log.InfoLevel, v, fields)
}

// Severe is go-zero's most serious level — used for unrecoverable process-level
// failures. Mapped to FatalLevel; go-spring's fatal path does NOT call os.Exit
// on its own (only the caller of logx.Severe does, if it chooses), so this is
// purely a level signal.
func (w *gsBridgeWriter) Severe(v any) {
	w.record(log.FatalLevel, v, nil)
}

// Slow marks a "slow" event (e.g. a slow SQL / slow call). No dedicated slow
// level in go-spring, so mapped to WarnLevel where operators typically watch
// for latency signals.
func (w *gsBridgeWriter) Slow(v any, fields ...logx.LogField) {
	w.record(log.WarnLevel, v, fields)
}

// Stack is used by logx to emit a stack trace at ERROR severity.
func (w *gsBridgeWriter) Stack(v any) {
	w.record(log.ErrorLevel, v, nil)
}

// Stat is periodic statistical output (e.g. logx's built-in stat lines).
// Mapped to InfoLevel; it is informational by intent, not a warning.
func (w *gsBridgeWriter) Stat(v any, fields ...logx.LogField) {
	w.record(log.InfoLevel, v, fields)
}
