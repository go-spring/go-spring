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

package log

import (
	"context"
	"os"
	"runtime"
	"time"
)

var (
	defaultLayout = &TextLayout{
		BaseLayout: BaseLayout{
			FileLineMaxLength: 48,
		},
	}

	// defaultLogger is the fallback logger that will be used if no custom logger
	// is configured for a specific tag.
	defaultLogger Logger = &ConsoleLogger{
		LoggerBase: LoggerBase{
			Level: LevelRange{
				MinLevel: defaultLogLevel(),
				MaxLevel: MaxLevel,
			},
		},
		Layout: defaultLayout,
		appender: &ConsoleAppender{
			AppenderBase: AppenderBase{
				Layout: defaultLayout,
			},
		},
	}

	// TagAppDef is the default tag for application-related logs.
	TagAppDef = RegisterAppTag("def", "")

	// TagBizDef is the default tag for business-related logs.
	TagBizDef = RegisterBizTag("def", "")

	// ReportError is an optional hook function that is invoked whenever an error
	// occurs during logging. It can be overridden to handle error reporting, such as
	// logging the error to a separate error log or sending alerts.
	ReportError = func(err error) {}

	// TimeNow is an optional override function that provides a custom timestamp.
	// It can be replaced during testing or in special cases where a fixed time
	// is required, ensuring consistency in log events across test runs.
	TimeNow func(ctx context.Context) time.Time

	// StringFromContext is an optional hook to extract a string (e.g., trace ID)
	// from the context. This string will be attached to the log event.
	// Avoid performing complex calculations in this function.
	// It's recommended to use cached results for better performance.
	StringFromContext func(ctx context.Context) string

	// FieldsFromContext is an optional hook to extract structured fields
	// (e.g., trace ID, span ID, or request metadata) from the context.
	// Avoid performing complex calculations in this function.
	// It's recommended to use cached results for better performance.
	FieldsFromContext func(ctx context.Context) []Field
)

// defaultLogLevel returns the default log level for the default logger.
// It checks the environment variable "GS_LOGGER_DEFAULT_LEVEL" and returns
// the corresponding log level. If the environment variable is not set,
// it defaults to InfoLevel.
func defaultLogLevel() Level {
	s, ok := os.LookupEnv("GS_LOGGER_DEFAULT_LEVEL")
	if !ok {
		return InfoLevel
	}
	r, err := ParseLevelRange(s)
	if err != nil {
		panic(err) // can panic here
	}
	return r.MinLevel
}

// RegisterAppTag registers or retrieves a Tag intended for application-layer logs,
// which are typically used to log events related to the application lifecycle,
// such as startup, shutdown, or health checks.
//   - subType: component or module name
//   - action: lifecycle phase or behavior (optional)
func RegisterAppTag(subType, action string) *Tag {
	return RegisterTag(BuildTag("app", subType, action))
}

// RegisterBizTag registers or retrieves a Tag intended for business-logic logs.
//   - subType: business domain or feature name
//   - action: operation being logged (optional)
func RegisterBizTag(subType, action string) *Tag {
	return RegisterTag(BuildTag("biz", subType, action))
}

// RegisterRPCTag registers or retrieves a Tag intended for RPC logs,
// covering external/internal dependency interactions.
//   - subType: protocol or target system (e.g., http, grpc, redis)
//   - action: RPC phase (e.g., send, retry, fail)
func RegisterRPCTag(subType, action string) *Tag {
	return RegisterTag(BuildTag("rpc", subType, action))
}

// getLogger returns the logger associated with the given tag.
// If no logger is bound, the default logger is returned.
func getLogger(tag *Tag) Logger {
	if l := tag.logger.Load().Logger; l != nil {
		return l
	}
	return defaultLogger
}

// Trace logs a message at TraceLevel using a lazy field generator.
// The generator function is only invoked if the level is enabled.
func Trace(ctx context.Context, tag *Tag, fn func() []Field) {
	if l := getLogger(tag); l.GetLevel().Enable(TraceLevel) {
		record(ctx, TraceLevel, tag.tag, l, 2, fn()...)
	}
}

// Tracef logs a formatted message at TraceLevel.
func Tracef(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(TraceLevel) {
		record(ctx, TraceLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Debug logs a message at DebugLevel using a lazy field generator.
// The generator function is only invoked if the level is enabled.
func Debug(ctx context.Context, tag *Tag, fn func() []Field) {
	if l := getLogger(tag); l.GetLevel().Enable(DebugLevel) {
		record(ctx, DebugLevel, tag.tag, l, 2, fn()...)
	}
}

// Debugf logs a formatted message at DebugLevel.
func Debugf(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(DebugLevel) {
		record(ctx, DebugLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Info logs structured fields at InfoLevel.
func Info(ctx context.Context, tag *Tag, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(InfoLevel) {
		record(ctx, InfoLevel, tag.tag, l, 2, fields...)
	}
}

// Infof logs a formatted message at InfoLevel.
func Infof(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(InfoLevel) {
		record(ctx, InfoLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Warn logs structured fields at WarnLevel.
func Warn(ctx context.Context, tag *Tag, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(WarnLevel) {
		record(ctx, WarnLevel, tag.tag, l, 2, fields...)
	}
}

// Warnf logs a formatted message at WarnLevel.
func Warnf(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(WarnLevel) {
		record(ctx, WarnLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Error logs structured fields at ErrorLevel.
func Error(ctx context.Context, tag *Tag, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(ErrorLevel) {
		record(ctx, ErrorLevel, tag.tag, l, 2, fields...)
	}
}

// Errorf logs a formatted message at ErrorLevel.
func Errorf(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(ErrorLevel) {
		record(ctx, ErrorLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Panic logs structured fields at PanicLevel.
func Panic(ctx context.Context, tag *Tag, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(PanicLevel) {
		record(ctx, PanicLevel, tag.tag, l, 2, fields...)
	}
}

// Panicf logs a formatted message at PanicLevel.
func Panicf(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(PanicLevel) {
		record(ctx, PanicLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Fatal logs structured fields at FatalLevel.
func Fatal(ctx context.Context, tag *Tag, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(FatalLevel) {
		record(ctx, FatalLevel, tag.tag, l, 2, fields...)
	}
}

// Fatalf logs a formatted message at FatalLevel.
func Fatalf(ctx context.Context, tag *Tag, format string, args ...any) {
	if l := getLogger(tag); l.GetLevel().Enable(FatalLevel) {
		record(ctx, FatalLevel, tag.tag, l, 2, Msgf(format, args...))
	}
}

// Record logs a message at the given level for the given tag.
func Record(ctx context.Context, level Level, tag *Tag, skip int, fields ...Field) {
	if l := getLogger(tag); l.GetLevel().Enable(level) {
		record(ctx, level, tag.tag, l, skip, fields...)
	}
}

// record performs the actual logging logic after level checking.
func record(ctx context.Context, level Level, tag string, logger Logger, skip int, fields ...Field) {
	var (
		file string
		line int
	)

	switch callerType {
	case CallerTypeDefault:
		_, file, line, _ = runtime.Caller(skip)
	case CallerTypeFast:
		file, line = FastCaller(skip)
	default: // for linter
	}

	now := time.Now()
	if TimeNow != nil {
		now = TimeNow(ctx)
	}

	var ctxString string
	if StringFromContext != nil {
		ctxString = StringFromContext(ctx)
	}

	var ctxFields []Field
	if FieldsFromContext != nil {
		ctxFields = FieldsFromContext(ctx)
	}

	e := getEvent()
	e.Level = level
	e.Time = now
	e.File = file
	e.Line = line
	e.Tag = tag
	e.Fields = fields
	e.CtxString = ctxString
	e.CtxFields = ctxFields
	logger.Append(e)
}
